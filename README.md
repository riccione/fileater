# Fileater

Fileater is a lightweight, high-performance CLI utility written in Go designed to organize cluttered directories into categorized folders. It follows the **KISS (Keep It Simple, Stupid)** principle, providing a fast and reliable way to manage files based on their extensions while keeping your directory structure clean.

## Features

* **Categorization**: Automatically sorts files into predefined folders like `video`, `audio`, `docs`, and `mix`.
* **Recursive Processing**: Clean up entire directory trees with a single command.
* **Smart Cleanup**: Automatically removes empty subdirectories after moving files to ensure a clean workspace.
* **Collision Resolution**: Prevents overwriting by automatically renaming files (e.g., `file.txt` -> `file_1.txt`) if a naming conflict occurs.
* **Dry Run Mode**: Preview all changes before they happen without modifying any files.
* **Atomic Operations**: Uses atomic renames with streaming copy fallbacks for cross-device moves.

## Installation

### Prerequisites
* [Go](https://go.dev/doc/install) (1.20 or later recommended)
* Git

### Building from Source
1.  Clone the repository:
    ```bash
    git clone https://github.com/youruser/fileater.git
    cd fileater
    ```
2.  Build the binary using the provided Makefile:
    ```bash
    make build
    ```
    The executable will be located in the `bin/` directory.

3.  (Optional) Run tests to ensure everything is set up correctly:
    ```bash
    make test
    ```

## Usage

Basic command pattern:
```bash
./bin/fileater [path] [flags]
```

### Examples

**Organize a single directory:**
```bash
./bin/fileater ~/Downloads
```

**Organize recursively with a dry run (simulation):**
```bash
./bin/fileater ~/Downloads -r --dryrun
```

**Force recursive organization (skips the confirmation prompt):**
```bash
./bin/fileater ~/Downloads -r -f
```

## Options & Flags

| Flag | Shorthand | Description |
| :--- | :--- | :--- |
| `--recursive` | `-r` | Process subdirectories recursively and move files to root categories. |
| `--force` | `-f` | Skip the confirmation prompt when using recursive mode. |
| `--dryrun` | `-d` | Simulate the operation without moving files or creating directories. |
| `--config` | `-c` | Path to a custom JSON configuration file (defaults to `config.json`). |
| `--version` | | Show the current version of Fileater. |
| `--help` | `-h` | Show help message with all available commands. |

## Configuration

Fileater uses a `config.json` file to map extensions to folder names. If no configuration file is found, it uses internal defaults for videos, audio, and documents.

### Example `config.json`
```json
{
  "images": [".jpg", ".jpeg", ".png", ".gif"],
  "code": [".go", ".py", ".rs", ".js"],
  "archives": [".zip", ".tar.gz", ".rar"]
}
```
*Note: Any file extension not defined in your configuration will be moved to the `mix` folder.*

## License

This project is licensed under the MIT License. See the `LICENSE` file for details.
