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

### 🎯 Use Cases

- `.git` source code leakage
- `.svn` source code leakage
- `.DS_Store` information leakage
- Directory listing exposure

## 🚀 Quick Start

### Installation

```bash
# Clone repository
git clone https://github.com/whgojp/DumpAll-Go.git

# Enter project directory
cd DumpAll-Go

# Install dependencies
go mod tidy

# Build
go build
```

### Usage

```bash
Usage:
  dumpall-go [flags]

Flags:
  -u, --url string      Target URL
  -f, --file string     File containing list of URLs
  -o, --outdir string   Output directory (default "output")
  -p, --proxy string    Proxy server (e.g., http://127.0.0.1:8080)
  -w, --workers int     Number of concurrent workers (default 10)
  -h, --help           Show help information
```

### Examples

1. Scan single target:
```bash
./dumpall-go -u http://example.com/
```

![Single Target Scan](./pic/url.png)

2. Batch scanning:
```bash
./dumpall-go -f target.txt
```

![Batch Scanning](./pic/file.png)

## 🤝 Contributing

We welcome all forms of contributions, including but not limited to:

- Submitting issues and suggestions
- Improving documentation
- Contributing code fixes or new features

## 📄 License

When we speak of free software, we are referring to freedom, not price.

This project is licensed under the [Apache License 2.0](LICENSE).

## 🙏 Acknowledgments

- Thanks to the original [DumpAll](https://github.com/0x727/DumpAll) project for inspiration
- Thanks to all contributors for their support 