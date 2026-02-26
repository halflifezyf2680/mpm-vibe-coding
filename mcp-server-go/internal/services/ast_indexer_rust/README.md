# Rust AST Indexer

Replacement for the legacy Go indexer, using Tree-sitter for high-accuracy AST analysis.

## Build

Requirements:
- Rust Toolchain (1.70+)
- C Compiler (MSVC `cl` or MinGW `gcc`)

```bash
cargo build --release
```

## Usage

```bash
ast_indexer_rust --mode index --project "C:/Project" --db "./symbols.db"
ast_indexer_rust --mode query --project "C:/Project" --db "./symbols.db" --query "my_func"
```

## Architecture

- **Tree-sitter**: Used for parsing (Python, JS, Go, Rust).
- **Rusqlite**: SQLite storage.
- **Rayon**: Parallel processing (future).
- **WalkDir**: Fast directory traversal.

## Design Decisions

- **Why Rust?**: No GC pauses, easier integration with Tree-sitter C libs via Cargo, type safety.
- **Static Linking**: The binary includes SQLite and Tree-sitter, zero runtime dependencies.
