package processor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractDateFromRINEX(t *testing.T) {
	// Создаем тестовый RINEX файл
	content := `     2.11           OBSERVATION DATA    M: Mixed            RINEX VERSION / TYPE
  2025    07    14    10    12   31.9999398     GPS         TIME OF FIRST OBS   
                                                            END OF HEADER       
 25 07 14 10 12 31.9999398  0 23G32G01G17G02G23G24G25G10G28E03E05E24`

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.obs")
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	processor := NewRINEXProcessor(1)
	header, err := processor.extractDateFromRINEX(testFile)
	require.NoError(t, err)

	assert.Equal(t, 2025, header.Year)
	assert.Equal(t, 7, header.Month)
	assert.Equal(t, 14, header.Day)
	assert.Equal(t, 10, header.Hour)
	assert.Equal(t, 12, header.Minute)
	assert.InDelta(t, 31.9999398, header.Second, 0.0000001)
}

func TestBuildEphemerisURL(t *testing.T) {
	header := &RINEXHeader{
		Year:   2025,
		Month:  7,
		Day:    14,
		Hour:   10,
		Minute: 12,
		Second: 31.9999398,
	}

	url := buildEphemerisURL(header)
	expected := "https://igs.bkg.bund.de/root_ftp/IGS/BRDC/2025/195/BRDC00IGS_R_20251950000_01D_MN.rnx.gz"
	assert.Equal(t, expected, url)
}

func TestParseSolution(t *testing.T) {
	content := `%  UTC                       x-ecef(m)      y-ecef(m)      z-ecef(m)   Q  ns   sdx(m)   sdy(m)   sdz(m)
2025/07/14 10:12:14.000    455582.3821   3639405.2210   5200656.2537   5   7   3.9804   7.7201  13.0986`

	tmpDir := t.TempDir()
	solFile := filepath.Join(tmpDir, "solution.pos")
	err := os.WriteFile(solFile, []byte(content), 0644)
	require.NoError(t, err)

	result, err := parseSolution(solFile)
	require.NoError(t, err)

	assert.InDelta(t, 455582.3821, result.X, 0.0001)
	assert.InDelta(t, 3639405.2210, result.Y, 0.0001)
	assert.InDelta(t, 5200656.2537, result.Z, 0.0001)
	assert.InDelta(t, 3.9804, result.SDX, 0.0001)
	assert.InDelta(t, 7.7201, result.SDY, 0.0001)
	assert.InDelta(t, 13.0986, result.SDZ, 0.0001)
}

func TestGunzipFile(t *testing.T) {
	// Создаем тестовый gz файл
	tmpDir := t.TempDir()
	src := filepath.Join(tmpDir, "test.gz")
	dst := filepath.Join(tmpDir, "test.txt")

	// Просто проверяем, что функция не паникует
	// В реальном тесте нужно создать настоящий gz файл
	err := gunzipFile(src, dst)
	assert.Error(t, err) // ожидаем ошибку, так как файл не существует
}
