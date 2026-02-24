package processor

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gnss-service/internal/interfaces"
	"gnss-service/internal/storage"

	"go.uber.org/zap"
)

const (
	rnx2rtkpPath = "./cmd/rtklib/app/rnx2rtkp"
	configPath   = "./cmd/rtklib/app/single.conf"
	tempDir      = "./tmp"
)

type RINEXProcessor struct {
	workerPool chan struct{}
}

type RINEXHeader struct {
	Year   int
	Month  int
	Day    int
	Hour   int
	Minute int
	Second float64
}

func NewRINEXProcessor(maxConcurrent int) *RINEXProcessor {
	os.MkdirAll(tempDir, 0755)

	return &RINEXProcessor{
		workerPool: make(chan struct{}, maxConcurrent),
	}
}

func (p *RINEXProcessor) ProcessFile(
	fileID int64,
	userLogin string,
	fileData []byte,
	dbStorage interfaces.Storage,
	logger *zap.SugaredLogger,
) {
	p.workerPool <- struct{}{}
	defer func() { <-p.workerPool }()

	logger.Infof("Starting processing for file ID: %d, user: %s", fileID, userLogin)

	dbStorage.UpdateFileStatus(fileID, "processing", nil)

	workDir := filepath.Join(tempDir, fmt.Sprintf("job_%d_%d", fileID, time.Now().UnixNano()))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		logger.Errorf("Failed to create work dir: %v", err)
		dbStorage.UpdateFileStatus(fileID, "failed", nil)
		return
	}
	defer func() {
		os.RemoveAll(workDir)
		logger.Infof("Cleaned up work directory: %s", workDir)
	}()

	// 1. Сохраняем файл наблюдений
	obsPath := filepath.Join(workDir, fmt.Sprintf("%s_%d.obs", userLogin, fileID))
	if err := os.WriteFile(obsPath, fileData, 0644); err != nil {
		logger.Errorf("Failed to save observation file: %v", err)
		dbStorage.UpdateFileStatus(fileID, "failed", nil)
		return
	}
	logger.Infof("Saved observation file: %s", obsPath)

	// 2. Определяем дату из файла
	date, err := p.extractDateFromRINEX(obsPath)
	if err != nil {
		logger.Errorf("Failed to extract date: %v", err)
		dbStorage.UpdateFileStatus(fileID, "failed", nil)
		return
	}
	logger.Infof("Extracted date: %04d-%02d-%02d %02d:%02d:%06.3f",
		date.Year, date.Month, date.Day, date.Hour, date.Minute, date.Second)

	// 3. Формируем URL для скачивания эфемерид
	navURL := buildEphemerisURL(date)
	navPath := filepath.Join(workDir, "navfile.rnx.gz")

	logger.Infof("Downloading ephemeris from: %s", navURL)

	// 4. Скачиваем файл эфемерид
	if err := downloadFile(navURL, navPath); err != nil {
		logger.Errorf("Failed to download ephemeris: %v", err)
		dbStorage.UpdateFileStatus(fileID, "failed", nil)
		return
	}
	logger.Infof("Downloaded ephemeris file: %s", navPath)

	// 5. Распаковываем .gz файл
	navUnpacked := filepath.Join(workDir, "navfile.rnx")
	if err := gunzipFile(navPath, navUnpacked); err != nil {
		logger.Errorf("Failed to unpack ephemeris: %v", err)
		dbStorage.UpdateFileStatus(fileID, "failed", nil)
		return
	}
	logger.Infof("Unpacked ephemeris to: %s", navUnpacked)

	// 6. Запускаем rnx2rtkp
	solutionPath := filepath.Join(workDir, "solution.pos")

	// Проверяем, что файлы существуют
	if _, err := os.Stat(obsPath); err != nil {
		logger.Errorf("Observation file not found: %v", err)
		dbStorage.UpdateFileStatus(fileID, "failed", nil)
		return
	}

	if _, err := os.Stat(navUnpacked); err != nil {
		logger.Errorf("Navigation file not found: %v", err)
		dbStorage.UpdateFileStatus(fileID, "failed", nil)
		return
	}

	// Формируем команду
	cmd := exec.Command(rnx2rtkpPath,
		"-k", configPath,
		"-o", solutionPath,
		obsPath, navUnpacked)

	logger.Infof("Running command: %s -k %s -o %s %s %s",
		rnx2rtkpPath, configPath, solutionPath, obsPath, navUnpacked)

	// Запускаем и получаем вывод
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("rnx2rtkp failed: %v", err)
		logger.Errorf("Output: %s", string(output))
		dbStorage.UpdateFileStatus(fileID, "failed", nil)
		return
	}

	logger.Infof("rnx2rtkp output: %s", string(output))
	logger.Infof("rnx2rtkp completed successfully")

	// 7. Проверяем, что solution файл создан
	if _, err := os.Stat(solutionPath); err != nil {
		logger.Errorf("Solution file not created: %v", err)
		dbStorage.UpdateFileStatus(fileID, "failed", nil)
		return
	}

	// 8. Парсим результат из solution.pos
	result, err := parseSolution(solutionPath)
	if err != nil {
		logger.Errorf("Failed to parse solution: %v", err)
		dbStorage.UpdateFileStatus(fileID, "failed", nil)
		return
	}

	result.FileID = fileID
	result.UserLogin = userLogin

	// 9. Сохраняем результат в БД
	resultID, err := dbStorage.SaveResult(result)
	if err != nil {
		logger.Errorf("Failed to save result to DB: %v", err)
		dbStorage.UpdateFileStatus(fileID, "failed", nil)
		return
	}

	// 10. Обновляем статус файла
	dbStorage.UpdateFileStatus(fileID, "completed", &resultID)

	logger.Infof("File %d processed successfully. Result ID: %d", fileID, resultID)
	logger.Infof("Coordinates: X=%.3f, Y=%.3f, Z=%.3f", result.X, result.Y, result.Z)
	logger.Infof("RMS: SDX=%.3f, SDY=%.3f, SDZ=%.3f", result.SDX, result.SDY, result.SDZ)
}

func (p *RINEXProcessor) extractDateFromRINEX(filePath string) (*RINEXHeader, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Сначала ищем TIME OF FIRST OBS в заголовке
	for scanner.Scan() {
		line := scanner.Text()

		// Ищем строку с "TIME OF FIRST OBS"
		if strings.Contains(line, "TIME OF FIRST OBS") {
			// Формат: "2025    07    14    10    12   31.9999398     GPS         TIME OF FIRST OBS"
			parts := strings.Fields(line)
			if len(parts) >= 6 {
				year, _ := strconv.Atoi(parts[0])
				month, _ := strconv.Atoi(parts[1])
				day, _ := strconv.Atoi(parts[2])
				hour, _ := strconv.Atoi(parts[3])
				min, _ := strconv.Atoi(parts[4])
				sec, _ := strconv.ParseFloat(parts[5], 64)

				// Для RINEX 2.x год может быть 2-значным
				if year < 100 {
					if year > 80 {
						year += 1900
					} else {
						year += 2000
					}
				}

				return &RINEXHeader{
					Year:   year,
					Month:  month,
					Day:    day,
					Hour:   hour,
					Minute: min,
					Second: sec,
				}, nil
			}
		}

		if strings.Contains(line, "END OF HEADER") {
			break
		}
	}

	file.Seek(0, 0)
	scanner = bufio.NewScanner(file)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "END OF HEADER") {
			break
		}
	}

	if scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 6 {
			year, _ := strconv.Atoi(parts[0])
			month, _ := strconv.Atoi(parts[1])
			day, _ := strconv.Atoi(parts[2])
			hour, _ := strconv.Atoi(parts[3])
			min, _ := strconv.Atoi(parts[4])
			sec, _ := strconv.ParseFloat(parts[5], 64)
			if year < 100 {
				if year > 80 {
					year += 1900
				} else {
					year += 2000
				}
			}

			return &RINEXHeader{
				Year:   year,
				Month:  month,
				Day:    day,
				Hour:   hour,
				Minute: min,
				Second: sec,
			}, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan file: %w", err)
	}

	return nil, fmt.Errorf("no observation data found in file")
}

func buildEphemerisURL(date *RINEXHeader) string {
	t := time.Date(date.Year, time.Month(date.Month), date.Day, 0, 0, 0, 0, time.UTC)
	doy := t.YearDay()

	doyStr := fmt.Sprintf("%03d", doy)

	filename := fmt.Sprintf("BRDC00IGS_R_%d%s0000_01D_MN.rnx.gz", date.Year, doyStr)

	return fmt.Sprintf("https://igs.bkg.bund.de/root_ftp/IGS/BRDC/%d/%s/%s",
		date.Year, doyStr, filename)
}

func downloadFile(url, destPath string) error {
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP GET failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	return nil
}

func gunzipFile(src, dst string) error {
	cmd := exec.Command("gunzip", "-c", src)
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	cmd.Stdout = out
	return cmd.Run()
}

func parseSolution(solutionPath string) (*storage.AdjustmentResult, error) {
	file, err := os.Open(solutionPath)
	if err != nil {
		return nil, fmt.Errorf("open solution file: %w", err)
	}
	defer file.Close()

	result := &storage.AdjustmentResult{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "%") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 10 {
			continue
		}

		x, err1 := strconv.ParseFloat(parts[2], 64)
		y, err2 := strconv.ParseFloat(parts[3], 64)
		z, err3 := strconv.ParseFloat(parts[4], 64)

		if err1 == nil && err2 == nil && err3 == nil {
			result.X = x
			result.Y = y
			result.Z = z

			if len(parts) >= 10 {
				if sdx, err := strconv.ParseFloat(parts[7], 32); err == nil {
					result.SDX = float32(sdx)
				}
				if sdy, err := strconv.ParseFloat(parts[8], 32); err == nil {
					result.SDY = float32(sdy)
				}
				if sdz, err := strconv.ParseFloat(parts[9], 32); err == nil {
					result.SDZ = float32(sdz)
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan solution: %w", err)
	}

	if result.X == 0 && result.Y == 0 && result.Z == 0 {
		return nil, fmt.Errorf("no valid coordinates found in solution file")
	}

	return result, nil
}
