package storage

import "sync"

type MemStorageAdj struct {
	estCoor  map[string]float32
	rmsInter map[string]float32
	mu       sync.RWMutex
}

type MemStorageUser struct {
	login       string
	password    string
	adjustments MemStorageAdj
	mu          sync.RWMutex
}

func NewMemStorage() *MemStorageAdj {
	return &MemStorageAdj{
		estCoor:  make(map[string]float32),
		rmsInter: make(map[string]float32),
	}
}

func NewUserStorage() *MemStorageUser {
	return &MemStorageUser{
		login:       "user",
		password:    "password",
		adjustments: *NewMemStorage(),
	}
}
