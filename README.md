# JSON Translator

A high-performance concurrent JSON translation CLI tool written in Go. It automatically translates localization/i18n JSON files into 80+ target languages using Google Translate while preserving key ordering, nested objects/arrays, and dynamic placeholders (e.g. `${variable}`).

## Features

- **Concurrent Processing**: Multi-worker architecture translates multiple locale files in parallel.
- **Placeholder Protection**: Safely preserves template placeholders (e.g., `${userName}`, `${count}`) during translation.
- **Preserved Key Ordering**: Maintains key insertion order using ordered maps.
- **Incremental Sync & Resume**: Supports checkpoints, incremental updates, and automatic recovery from interrupted runs.
- **Multi-locale Support**: Generates localization files for 80+ language and regional codes into the `locale/` directory.
- **Post-processing Safety Net**: Scans generated files to ensure all template variables remain intact.

## Getting Started

### Prerequisites

- [Go](https://go.dev/doc/install) 1.20+ installed.

### Installation

Clone the repository and install dependencies:

```bash
git clone https://github.com/YazadDumasia/json-translator.git
cd json-translator
go mod download
```

### Usage

1. Place your base source English JSON file named `en.json` in the project root:

   ```json
   {
     "welcome": "Welcome back, ${name}!",
     "app": {
       "title": "My Awesome App",
       "description": "Translate your app effortlessly."
     }
   }
   ```

2. Run the translator:

   ```bash
   go run main.go
   ```

   Or build and execute the binary:

   ```bash
   go build -o json-translator main.go
   ./json-translator
   ```

3. The generated locale files will be exported to the `locale/` folder (e.g., `locale/locale_fr.json`, `locale/locale_es.json`, `locale/locale_hi.json`, etc.).

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

## License

This project is licensed under the MIT License.
