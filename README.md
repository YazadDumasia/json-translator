# JSON Translator

A high-performance concurrent JSON translation CLI tool written in Go. It automatically translates localization/i18n JSON files into 80+ target languages using Google Translate while preserving key ordering, nested objects/arrays, and dynamic placeholders (e.g. `${variable}`).

## Features

- **Concurrent Processing**: Multi-worker architecture (10 parallel goroutines) translates multiple locale files simultaneously.
- **Placeholder Protection**: Safely preserves template placeholders (e.g., `${userName}`, `${count}`) during translation using temporary tokens.
- **Preserved Key Ordering**: Maintains key insertion order using ordered maps.
- **Incremental Sync & Resume**: Supports checkpoints, incremental updates, and automatic recovery from interrupted runs.
- **Multi-locale Support**: Generates localization files for 80+ language and regional codes into the `locale/` directory.
- **Post-processing Safety Net**: Scans generated files to ensure all template variables remain intact.

---

## Technical Architecture & Code Flow

1. **Imports & Order Preservation (`main.go`)**:
   Uses standard library packages (`net/http`, `sync`, `regexp`, `os`) alongside `github.com/iancoleman/orderedmap` to guarantee key ordering is maintained in all output JSON files.
2. **Master Sync (`syncMasterEnglish`)**:
   Verifies and synchronizes `en.json` with `locale/locale_en.json`.
3. **Worker Pool Parallelization**:
   Spawns 10 concurrent worker goroutines with `sync.WaitGroup` to translate languages simultaneously.
4. **Placeholder Protection (`translateWithPlaceholderProtection`)**:
   Replaces `${...}` expressions with temporary `__VAR_X__` tokens, executes API calls, and restores the original placeholders.
5. **Atomic Disk Operations (`writeOrderedJSON`)**:
   Writes progress to `.tmp` files before renaming to prevent corrupted state on unexpected interrupts. Every 20 translated keys create a checkpoint.
6. **Post-Processing Audit (`restorePlaceholdersAcrossAllFiles`)**:
   Compares all translated strings against the master file to fix any lost variable placeholders.

---

## Installation Guide

### Prerequisites

- [Go](https://go.dev/doc/install) 1.20 or higher installed.

Verify Go installation:
```bash
go version
```

### Step-by-Step Installation

1. **Clone the repository**:
   ```bash
   git clone https://github.com/YazadDumasia/json-translator.git
   cd json-translator
   ```

2. **Download dependencies**:
   ```bash
   go mod download
   ```

3. **Prepare source file**:
   Create your `en.json` file in the root directory:
   ```json
   {
     "welcome": "Welcome back, ${name}!",
     "app": {
       "title": "My Awesome App",
       "description": "Translate your app effortlessly."
     }
   }
   ```

4. **Run the translator**:
   ```bash
   go run main.go
   ```

   *Or compile into an executable binary:*
   ```bash
   go build -o json-translator main.go
   ./json-translator
   ```

---

## Output Directory Structure

```text
json-translator/
├── en.json              # Base English source file
├── main.go              # Application entrypoint
├── locale/              # Output directory for localized JSON files
│   ├── locale_en.json   # Master English file
│   ├── locale_es.json   # Spanish translation
│   ├── locale_fr.json   # French translation
│   ├── locale_hi.json   # Hindi translation
│   └── ...
└── translation.log      # Execution and checkpoint log file
```

---

## License

This project is licensed under the MIT License.
