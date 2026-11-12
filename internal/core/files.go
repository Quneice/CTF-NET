// 文件存储：管理上传的文件（对应 legacy/app/storage/files.py）。
package core

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
)

// StoredFile 已存储的上传文件。
type StoredFile struct {
	ID   string
	Name string
	Path string
	Size int64
}

// FileStore 上传文件仓库：<id> 存内容，<id>.name 存原始文件名。
type FileStore struct {
	BaseDir string
}

// NewFileStore 创建存储并确保目录存在（对应 Python __init__ mkdir）。
func NewFileStore(baseDir string) *FileStore {
	_ = os.MkdirAll(baseDir, 0o755)
	return &FileStore{BaseDir: baseDir}
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Save 保存文件，返回存储记录（id 为 16 位 hex，与 uuid4().hex[:16] 对齐）。
func (fs *FileStore) Save(filename string, data []byte) (*StoredFile, error) {
	fid := randomHex(8)
	path := filepath.Join(fs.BaseDir, fid)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, err
	}
	_ = os.WriteFile(path+".name", []byte(filename), 0o644)
	return &StoredFile{ID: fid, Name: filename, Path: path, Size: int64(len(data))}, nil
}

// Get 按 file_id 取回；不存在返回 nil（对应 Python 返回 None）。
func (fs *FileStore) Get(fid string) (*StoredFile, error) {
	path := filepath.Join(fs.BaseDir, fid)
	st, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	name := "file"
	if b, err := os.ReadFile(path + ".name"); err == nil {
		name = string(b)
	}
	return &StoredFile{ID: fid, Name: name, Path: path, Size: st.Size()}, nil
}
