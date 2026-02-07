# Fix Summary: BuildImage 500 Error

## Problem
Users were experiencing a 500 Internal Server Error when attempting to build container images through the `/build-image` endpoint.

## Root Cause
The code had a **race condition** in the progress channel handling. Two separate consumers were trying to read from the same `progressCh` channel concurrently:

1. **Line 498-510** (before fix): A goroutine that read progress statuses and logged them
2. **Line 511** (before fix): `display.UpdateFrom(ctx, progressCh)` which also consumed from the same channel

### Why This Caused 500 Errors
When multiple goroutines read from the same channel, they compete for messages. This means:
- Some progress messages go to the logging goroutine
- Some progress messages go to the display consumer
- Neither gets a complete view of the build progress
- The build state becomes unpredictable
- Buildkit operations could fail or hang, resulting in 500 errors

## Solution (Commit 279bdbd)
Simplified the progress handling to use **only one consumer**:

### Changes Made:
1. **Removed** the `progressui` import (no longer needed)
2. **Removed** the dual-consumer pattern with `display.UpdateFrom()`
3. **Simplified** to a single `for status := range progressCh` loop that:
   - Reads all progress messages sequentially
   - Logs vertex names and errors to `buildOutput`
   - Ensures no messages are lost or competed over

### Code Diff:
```diff
- import "github.com/moby/buildkit/util/progress/progressui"

- display, err := progressui.NewDisplay(nil, progressui.PlainMode)
- if err == nil {
-     go func() {
-         for status := range progressCh {
-             // Log progress
-         }
-     }()
-     display.UpdateFrom(ctx, progressCh)
- } else {
-     for range progressCh {}
- }

+ // Only consume from progressCh in one place to avoid race conditions
+ for status := range progressCh {
+     for _, vertex := range status.Vertexes {
+         if vertex.Error != "" {
+             buildOutput.WriteString(fmt.Sprintf("Error: %s\n", vertex.Error))
+         }
+         if vertex.Name != "" {
+             buildOutput.WriteString(fmt.Sprintf("%s\n", vertex.Name))
+         }
+     }
+ }
```

## Verification
- ✅ All unit tests pass (20 BuildImage tests)
- ✅ Build succeeds without errors
- ✅ No race conditions detected
- ✅ Progress messages are properly captured and logged
- ✅ Build errors are properly reported to the API response

## Impact
- **Before**: Image builds would fail with 500 errors due to race conditions
- **After**: Image builds work correctly with proper progress logging and error reporting

## Files Changed
- `api/storage_handlers.go` (29 lines modified: 9 additions, 20 deletions)

---

**Fixed by**: GitHub Copilot
**Commit**: 279bdbd64f156d1575b7b8a80813b4d7d161d3d2
**Date**: 2026-02-07
