package paths

// MakeDirLink creates a directory link at `link` pointing at `target`.
//
//   - On Linux/macOS this is `os.Symlink` (unprivileged).
//   - On Windows it's an NTFS junction (also unprivileged, unlike a symlink
//     which would require admin or Developer Mode).
//
// `link` must not already exist; create-then-rename through a temp link is
// the caller's responsibility (see store.Use).
//
// RemoveDirLink removes a link previously created by MakeDirLink. On unix
// it's `os.Remove`; on Windows it removes the reparse point without touching
// the target directory.
