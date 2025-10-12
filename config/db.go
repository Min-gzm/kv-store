package config

type DBConfig struct {
	DataDirPath          string
	DataFileSize         int64
	SyncAfterEveryWrites bool
	BytesPerSync         int64
}
