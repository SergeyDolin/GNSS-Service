package processor

import (
	"math/rand"
	"time"

	"gnss-service/internal/storage"

	"go.uber.org/zap"
)

type RINEXProcessor struct {
	workerPool chan struct{}
}

func NewRINEXProcessor(maxConcurrent int) *RINEXProcessor {
	return &RINEXProcessor{
		workerPool: make(chan struct{}, maxConcurrent),
	}
}

func (p *RINEXProcessor) ProcessFile(
	fileID int64,
	userLogin string,
	fileData []byte,
	dbStorage *storage.DBStorage,
	logger *zap.SugaredLogger,
) {

	p.workerPool <- struct{}{}
	defer func() { <-p.workerPool }()

	logger.Infof("Processing file ID: %d for user: %s", fileID, userLogin)

	dbStorage.UpdateFileStatus(fileID, "processing", nil)

	logger.Infof("Processing %d bytes...", len(fileData))
	time.Sleep(time.Duration(rand.Intn(3)+2) * time.Second)

	result := &storage.AdjustmentResult{
		FileID:    fileID,
		UserLogin: userLogin,
		X:         1234567.89 + rand.Float64()*100,
		Y:         1234567.89 + rand.Float64()*100,
		Z:         1234567.89 + rand.Float64()*100,
		SDX:       float32(rand.Float64() * 0.05),
		SDY:       float32(rand.Float64() * 0.05),
		SDZ:       float32(rand.Float64() * 0.05),
	}

	resultID, err := dbStorage.SaveResult(result)
	if err != nil {
		logger.Errorf("Failed to save result: %v", err)
		dbStorage.UpdateFileStatus(fileID, "failed", nil)
		return
	}

	dbStorage.UpdateFileStatus(fileID, "completed", &resultID)

	logger.Infof("File %d processed. Result saved with ID: %d", fileID, resultID)
}
