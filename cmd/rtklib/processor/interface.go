package processor

import "gnss-service/internal/interfaces"

type FileProcessor interface {
	ProcessFile(fileID int64, userLogin string, fileData []byte, storage interfaces.Storage, logger interface{}) error
}
