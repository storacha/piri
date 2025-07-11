package store

import (
	"github.com/storacha/piri/pkg/fx/store/filesystem"
	"github.com/storacha/piri/pkg/fx/store/memory"
)

// Module for full server - always uses filesystem stores
// The configuration (app.RepoConfig) will determine the actual paths used
var Module = filesystem.Module

// FileSystemStoreModule explicitly uses filesystem stores
var FileSystemStoreModule = filesystem.Module

// MemoryStoreModule explicitly uses memory stores  
var MemoryStoreModule = memory.Module
