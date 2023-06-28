package os

import (
	"context"
	"os"
)

type SimpleOS struct {
	ctx context.Context
}

func NewSimpleOS(ctx context.Context) *SimpleOS {
	return &SimpleOS{ctx: ctx}
}

func (osObj *SimpleOS) Chdir(dir string) error {
	return os.Chdir(dir)
}

func (osObj *SimpleOS) Create(name string) (File, error) {
	return os.Create(name)
}

func (osObj *SimpleOS) Environ() []string {
	return os.Environ()
}

func (osObj *SimpleOS) Exit(code int) {
	os.Exit(code)
}

func (osObj *SimpleOS) Getenv(key string) string {
	return os.Getenv(key)
}

func (osObj *SimpleOS) Getpid() int {
	return os.Getpid()
}

func (osObj *SimpleOS) Getuid() int {
	return os.Getuid()
}

func (osObj *SimpleOS) Getwd() (string, error) {
	return os.Getwd()
}

func (osObj *SimpleOS) Hostname() (string, error) {
	return os.Hostname()
}

func (osObj *SimpleOS) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

func (osObj *SimpleOS) Mkdir(name string, perm FileMode) error {
	return os.Mkdir(name, perm)
}

func (osObj *SimpleOS) MkdirAll(path string, perm FileMode) error {
	return os.MkdirAll(path, perm)
}

func (osObj *SimpleOS) MkdirTemp(dir, pattern string) (string, error) {
	return os.MkdirTemp(dir, pattern)
}

func (osObj *SimpleOS) Open(name string) (File, error) {
	return os.Open(name)
}

func (osObj *SimpleOS) OpenFile(name string, flag int, perm FileMode) (File, error) {
	return os.OpenFile(name, flag, perm)
}

func (osObj *SimpleOS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (osObj *SimpleOS) Remove(name string) error {
	return os.Remove(name)
}

func (osObj *SimpleOS) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (osObj *SimpleOS) Setenv(key, value string) error {
	return os.Setenv(key, value)
}

func (osObj *SimpleOS) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

func (osObj *SimpleOS) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

func (osObj *SimpleOS) TempDir() string {
	return os.TempDir()
}

func (osObj *SimpleOS) Unsetenv(key string) error {
	return os.Unsetenv(key)
}

func (osObj *SimpleOS) UserCacheDir() (string, error) {
	return os.UserCacheDir()
}

func (osObj *SimpleOS) UserConfigDir() (string, error) {
	return os.UserConfigDir()
}

func (osObj *SimpleOS) UserHomeDir() (string, error) {
	return os.UserHomeDir()
}

func (osObj *SimpleOS) WriteFile(name string, data []byte, perm FileMode) error {
	return os.WriteFile(name, data, perm)
}
