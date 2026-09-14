# Package: `store`

The data-access layer: owns all reads and writes to the single `fantasy-hockey.yml` data file (AD-9). No other package touches that file directly.

## Responsibilities

- `New(path)` loads `path` into memory, bootstrap-creating it with an empty `players` list and the current default season if it doesn't exist yet (AD-25, AD-26).
- `FindPlayerByEmail` looks up a hand-maintained `Player` by email.
- `CreateLoginCode` appends a new `LoginCode` row - one per issued code, hashed (`code_hash`), never plaintext - and persists it. Existing rows are never mutated or removed.

## Design notes

- `Store`'s in-memory document and mutex stay unexported; every access goes through an exported method (AD-29).
- Every write serializes the whole in-memory document and atomically replaces the file on disk via write-to-temp-file-then-rename (AD-27).
- `Player`/`LoginCode` are the canonical structs for these entities; other packages import and use them as-is (AD-24).
