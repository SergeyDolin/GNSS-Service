// internal/interfaces/storage.go
package interfaces

import "gnss-service/internal/storage"

type UserStorage interface {
	CreateUser(login, password string) error
	GetUser(login string) (*storage.User, error)
	UserExists(login string) (bool, error)
}

type FileStorage interface {
	CreateFile(userLogin, filename string, fileSize int64) (*storage.ObsFile, error)
	GetFile(fileID int64) (*storage.ObsFile, error)
	GetUserFiles(userLogin string) ([]*storage.ObsFile, error)
	UpdateFileStatus(fileID int64, status string, resultID *int64) error
}

type ResultStorage interface {
	SaveResult(result *storage.AdjustmentResult) (int64, error)
	GetResult(resultID int64) (*storage.AdjustmentResult, error)
	GetUserResults(userLogin string) ([]*storage.AdjustmentResult, error)
	GetLastUserResult(userLogin string) (*storage.AdjustmentResult, error)
}

type Storage interface {
	UserStorage
	FileStorage
	ResultStorage
	Close() error
}
