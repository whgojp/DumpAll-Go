# DumpAll-Go

[![Go Version](https://img.shields.io/github/go-mod/go-version/whgojp/DumpAll-Go)](https://github.com/whgojp/DumpAll-Go)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

English | [简体中文](README.md)

## 📖 Introduction

DumpAll-Go is a Go language reconstruction of [DumpAll](https://github.com/0x727/DumpAll), designed for automated collection and extraction of website sensitive information. This project maintains the original functionality while implementing comprehensive optimizations and improvements.

### ✨ Key Features

- 🚀 High Performance: Developed in Go language for superior execution efficiency
- 🌍 Cross-Platform: Support for Windows, Linux, macOS, and other major operating systems
- 🎯 Smart Detection: Automatic identification of various information leak types
- 📦 Ready to Use: No complex environment configuration required
- 🔄 Concurrent Processing: Support for batch scanning of multiple targets
- 🛡️ Reliable: Enhanced error tolerance and stability
- 🌐 Proxy Support: HTTP / HTTPS / SOCKS5 / SOCKS5H proxy support
- 🔓 SVN Source Extraction: One-click restore of full source code from `.svn` leakage

### 🎯 Use Cases

- `.git` source code leakage
- `.svn` source code leakage + one-click extraction
- `.DS_Store` information leakage
- Directory listing exposure

## 🚀 Quick Start

### Installation

#### Method 1: Download Binary

Download the appropriate binary from the [Releases](https://github.com/whgojp/DumpAll-Go/releases) page:

- Windows: `dumpall-go-windows-amd64.exe` or `dumpall-go-windows-386.exe`
- Linux: `dumpall-go-linux-amd64` or `dumpall-go-linux-386` or `dumpall-go-linux-arm64`
- macOS: `dumpall-go-darwin-amd64` or `dumpall-go-darwin-arm64`

#### Method 2: Build from Source

```bash
# Clone repository
git clone https://github.com/whgojp/DumpAll-Go.git

# Enter project directory
cd DumpAll-Go

# Install dependencies
make deps

# Build for all platforms
make all

# Or build for current platform only
make build

# Or build for specific platform
make build-windows  # Build for Windows
make build-linux    # Build for Linux
make build-darwin   # Build for macOS
```

The compiled binaries will be in the `build` directory.

### Usage

#### Default Scan Mode

```bash
Usage:
  dumpall-go [flags]

Flags:
  -u, --url string      Target URL
  -f, --file string     File containing list of URLs
  -o, --outdir string   Output directory (default "output")
  -p, --proxy string    Proxy server (supports: http://host:port | https://host:port | socks5://host:port | socks5h://host:port)
  -w, --workers int     Number of concurrent workers (default 10)
  -h, --help           Show help information
```

#### svn-extract — SVN Source Code Extraction

When `.svn` information leakage is confirmed on the target, use the `svn-extract` subcommand to directly extract and restore the complete source code.

```bash
Usage:
  dumpall-go svn-extract [flags]

Flags:
  -u, --url string      Target URL (required, e.g. http://example.com/)
  -o, --outdir string   Output directory (default: output/<hostname>)
  -p, --proxy string    Proxy server (supports: http://host:port | socks5://host:port | socks5h://host:port)
  -w, --workers int     Number of concurrent download workers (default 10)
  -h, --help           Show help information
```

Supports two SVN formats:
- **SVN 1.6 and earlier**: Parses `.svn/entries`, extracts files from `.svn/text-base/*.svn-base`
- **SVN 1.7 and above**: Parses `.svn/wc.db` (SQLite), downloads files directly

Output directory structure:
```
output/<hostname>/
  ├── .svn/           ← cached SVN metadata
  └── extracted/      ← ✅ restored source code (matches original project structure)
```

### Examples

1. Scan single target:
```bash
./dumpall-go -u http://example.com/
```

2. Batch scanning:
```bash
./dumpall-go -f target.txt
```

3. Scan with HTTP proxy:
```bash
./dumpall-go -u http://example.com/ -p http://127.0.0.1:8080
```
4. Scan with SOCKS5 proxy:
```bash
./dumpall-go -u http://example.com/ -p socks5://127.0.0.1:1080
```
5. Scan with authenticated SOCKS5 proxy:
```bash
./dumpall-go -u http://example.com/ -p socks5://user:pass@127.0.0.1:1080
```
6. Scan with SOCKS5H proxy (DNS resolved by proxy server):
```bash
./dumpall-go -u http://example.com/ -p socks5h://127.0.0.1:1080
```
7. Extract SVN leaked source code:
```bash
./dumpall-go svn-extract -u http://example.com/
```
8. Extract SVN source code via SOCKS5 proxy with custom output directory:
```bash
./dumpall-go svn-extract -u http://example.com/ -p socks5://127.0.0.1:1080 -o ./leaked-src
```

## 🤝 Contributing

We welcome all forms of contributions, including but not limited to:

- Submitting issues and suggestions
- Improving documentation
- Contributing code fixes or new features

## 📄 License

When we speak of free software, we are referring to freedom, not price.

This project is licensed under the [Apache License 2.0](LICENSE).
