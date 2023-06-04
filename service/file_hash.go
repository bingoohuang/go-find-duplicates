package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"hash/crc32"
	"os"

	"github.com/m-manu/go-find-duplicates/bytesutil"
	"github.com/m-manu/go-find-duplicates/entity"
	"github.com/m-manu/go-find-duplicates/utils"
	"github.com/samber/lo"
)

const (
	thresholdFileSize = 16 * bytesutil.KIBI
)

// GetDigest generates entity.FileDigest of the file provided
func GetDigest(path string, isThorough bool) (entity.FileDigest, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return entity.FileDigest{}, err
	}
	h, err := fileHash(path, isThorough)
	if err != nil {
		return entity.FileDigest{}, err
	}

	return entity.FileDigest{
		FileExtension: utils.GetFileExt(path),
		FileSize:      info.Size(),
		FileHash:      h,
	}, nil
}

// fileHash calculates the hash of the file provided.
// If isThorough is true, then it uses SHA256 of the entire file.
// Otherwise, it uses CRC32 of "crucial bytes" of the file.
func fileHash(path string, isThorough bool) (string, error) {
	fileInfo, statErr := os.Lstat(path)
	if statErr != nil {
		return "", fmt.Errorf("couldn't stat: %w", statErr)
	}
	if !fileInfo.Mode().IsRegular() {
		return "", fmt.Errorf("can't compute hash of non-regular file")
	}
	var prefix string
	var bytes []byte
	var fileReadErr error
	switch {
	case isThorough:
		bytes, fileReadErr = os.ReadFile(path)
	case fileInfo.Size() <= thresholdFileSize:
		prefix = "f"
		bytes, fileReadErr = os.ReadFile(path)
	default:
		prefix = "s"
		bytes, fileReadErr = readCrucialBytes(path, fileInfo.Size())
	}
	if fileReadErr != nil {
		return "", fmt.Errorf("couldn't calculate hash: %w", fileReadErr)
	}

	h := lo.TernaryF(isThorough, sha256.New, func() hash.Hash { return crc32.NewIEEE() })

	if _, err := h.Write(bytes); err != nil {
		return "", fmt.Errorf("error while computing hash: %w", err)
	}
	hashBytes := h.Sum(nil)
	return prefix + hex.EncodeToString(hashBytes), nil
}

// readCrucialBytes reads the first few bytes, middle bytes and last few bytes of the file
func readCrucialBytes(filePath string, fileSize int64) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	firstBytes := make([]byte, thresholdFileSize/2)
	if _, err := file.ReadAt(firstBytes, 0); err != nil {
		return nil, fmt.Errorf("couldn't read first few bytes (maybe file is corrupted?): %w", err)
	}
	middleBytes := make([]byte, thresholdFileSize/4)
	if _, err := file.ReadAt(middleBytes, fileSize/2); err != nil {
		return nil, fmt.Errorf("couldn't read middle bytes (maybe file is corrupted?): %w", err)
	}
	lastBytes := make([]byte, thresholdFileSize/4)
	if _, err := file.ReadAt(lastBytes, fileSize-thresholdFileSize/4); err != nil {
		return nil, fmt.Errorf("couldn't read end bytes (maybe file is corrupted?): %w", err)
	}
	bytes := append(append(firstBytes, middleBytes...), lastBytes...)
	return bytes, nil
}
