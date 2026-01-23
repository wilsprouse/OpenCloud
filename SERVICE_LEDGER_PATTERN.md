# Service Ledger Update Pattern for Functions

## Overview
This document explains how the Service Ledger update pattern works in OpenCloud, using pipelines as the reference implementation, so the same pattern can be applied to functions.

## Architecture

### Service Ledger Structure (`service_ledger/serviceLedger.go`)

The Service Ledger is a centralized JSON file that tracks the state of all OpenCloud services. It's located at `service_ledger/serviceLedger.json` and provides:

1. **Mutex-protected operations** - All updates are thread-safe via `ledgerMutex`
2. **Service-specific data structures** - `FunctionEntry` and `PipelineEntry`
3. **CRUD operations** - Create, Read, Update, Delete for each service type
4. **Service enablement tracking** - Each service has an `Enabled` flag

### Key Data Structures

```go
// Service Ledger Root
type ServiceLedger map[string]ServiceStatus

// Service Status
type ServiceStatus struct {
    Enabled   bool                      `json:"enabled"`
    Functions map[string]FunctionEntry  `json:"functions,omitempty"`
    Pipelines map[string]PipelineEntry  `json:"pipelines,omitempty"`
}

// Function Entry
type FunctionEntry struct {
    Runtime  string        `json:"runtime"`
    Trigger  string        `json:"trigger,omitempty"`
    Schedule string        `json:"schedule,omitempty"`
    Content  string        `json:"content"`
    Logs     []FunctionLog `json:"logs,omitempty"`
}

// Pipeline Entry (for reference)
type PipelineEntry struct {
    ID          string `json:"id"`
    Name        string `json:"name"`
    Description string `json:"description"`
    Code        string `json:"code"`
    Branch      string `json:"branch"`
    Status      string `json:"status"`
    CreatedAt   string `json:"createdAt"`
}
```

## Service Ledger Functions Available

### Generic Service Operations
- `ReadServiceLedger()` - Reads the entire ledger
- `WriteServiceLedger(ledger)` - Writes the entire ledger (atomic operation)
- `IsServiceEnabled(serviceName)` - Checks if a service is enabled
- `EnableService(serviceName)` - Enables a service

### Pipeline-Specific Operations (in serviceLedger.go)
- `UpdatePipelineEntry(id, name, desc, code, branch, status, createdAt)`
- `DeletePipelineEntry(pipelineID)`
- `GetPipelineEntry(pipelineID)`
- `GetAllPipelineEntries()`
- `SyncPipelines()` - Syncs filesystem to ledger

### Function-Specific Operations (in serviceLedger.go)
- `UpdateFunctionEntry(name, runtime, trigger, schedule, content)` ✅ Already exists
- `DeleteFunctionEntry(functionName)` ✅ Already exists
- `GetFunctionEntry(functionName)` ✅ Already exists
- `GetAllFunctionEntries()` ✅ Already exists

## Implementation Pattern

### 1. Frontend UI Pattern (React/Next.js)

#### Service Enablement Check
```typescript
// Check if service is enabled on page load
const checkServiceStatus = async () => {
  try {
    const res = await client.get<{ service: string; enabled: boolean }>(
      "/get-service-status?service=Functions"  // or "pipelines"
    )
    setServiceEnabled(res.data.enabled)
  } catch (err) {
    console.error("Failed to check service status:", err)
    setServiceEnabled(false)
  }
}

useEffect(() => {
  checkServiceStatus()
}, [])
```

#### Enable Service
```typescript
const handleEnableService = async () => {
  setEnablingService(true)
  try {
    await client.post("/enable-service", { service: "Functions" })
    setServiceEnabled(true)
    fetchFunctions()  // Load data after enabling
  } catch (err) {
    console.error("Failed to enable service:", err)
  } finally {
    setEnablingService(false)
  }
}
```

#### CRUD Operations
```typescript
// CREATE
const handleCreateFunction = async () => {
  await client.post("/create-function", { 
    name: functionName,
    runtime: functionRuntime,
    code: functionCode,
  })
  fetchFunctions()
}

// READ (List)
const fetchFunctions = async () => {
  const res = await client.get<FunctionItem[]>("/list-functions")
  setFunctions(res.data || [])
}

// UPDATE
const handleUpdateFunction = async (id: string) => {
  await client.put(`/update-function/${id}`, { 
    name: functionName,
    runtime: functionRuntime,
    code: functionCode,
  })
  fetchFunctions()
}

// DELETE
const handleDeleteFunction = async (id: string) => {
  await client.delete(`/delete-function?name=${id}`)
  fetchFunctions()
}
```

### 2. Backend API Pattern (Go)

#### Pattern for CreateFunction (api/compute_handlers.go)

```go
func CreateFunction(w http.ResponseWriter, r *http.Request) {
    // 1. Validate HTTP method
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // 2. Parse and validate request
    var req CreateFunctionRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    // 3. Validate required fields
    if req.Name == "" || req.Code == "" {
        http.Error(w, "Missing required fields", http.StatusBadRequest)
        return
    }

    // 4. Prepare filesystem paths
    home, err := os.UserHomeDir()
    functionDir := filepath.Join(home, ".opencloud", "functions")
    functionPath := filepath.Join(functionDir, functionFileName)

    // 5. Check if already exists (file and ledger)
    if _, err := os.Stat(functionPath); err == nil {
        http.Error(w, "Function already exists", http.StatusConflict)
        return
    }
    if existingEntry, err := service_ledger.GetFunctionEntry(functionFileName); 
       err == nil && existingEntry != nil {
        http.Error(w, "Function already exists in ledger", http.StatusConflict)
        return
    }

    // 6. Create filesystem entry
    if err := os.MkdirAll(functionDir, 0755); err != nil {
        http.Error(w, "Failed to create directory", http.StatusInternalServerError)
        return
    }
    if err := os.WriteFile(functionPath, []byte(req.Code), 0644); err != nil {
        http.Error(w, "Failed to create file", http.StatusInternalServerError)
        return
    }

    // 7. Update service ledger
    if err := service_ledger.UpdateFunctionEntry(
        functionFileName, 
        req.Runtime, 
        "", 
        "", 
        req.Code,
    ); err != nil {
        // Log but don't fail - file already created
        fmt.Printf("Warning: Failed to update service ledger: %v\n", err)
    }

    // 8. Return success response
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(responseObject)
}
```

#### Pattern for UpdateFunction

```go
func UpdateFunction(w http.ResponseWriter, r *http.Request) {
    // 1. Validate HTTP method
    if r.Method != http.MethodPut {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // 2. Extract ID from URL path
    id := strings.TrimPrefix(r.URL.Path, "/update-function/")
    if id == "" {
        http.Error(w, "Function ID required", http.StatusBadRequest)
        return
    }

    // 3. Parse request body
    var req UpdateFunctionRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    // 4. Verify function exists (file and optionally ledger)
    functionPath := filepath.Join(home, ".opencloud", "functions", id)
    if _, err := os.Stat(functionPath); os.IsNotExist(err) {
        http.Error(w, "Function not found", http.StatusNotFound)
        return
    }

    // 5. Update filesystem
    if err := os.WriteFile(functionPath, []byte(req.Code), 0644); err != nil {
        http.Error(w, "Failed to update file", http.StatusInternalServerError)
        return
    }

    // 6. Update service ledger
    if err := service_ledger.UpdateFunctionEntry(
        id, 
        req.Runtime, 
        trigger, 
        schedule, 
        req.Code,
    ); err != nil {
        fmt.Printf("Warning: Failed to update service ledger: %v\n", err)
    }

    // 7. Return success response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(responseObject)
}
```

#### Pattern for DeleteFunction

```go
func DeleteFunction(w http.ResponseWriter, r *http.Request) {
    // 1. Validate HTTP method
    if r.Method != http.MethodDelete {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // 2. Extract function name from query string
    fnName := r.URL.Query().Get("name")
    if fnName == "" {
        http.Error(w, "Function name required", http.StatusBadRequest)
        return
    }

    // 3. Verify function exists
    functionPath := filepath.Join(home, ".opencloud", "functions", fnName)
    if _, err := os.Stat(functionPath); os.IsNotExist(err) {
        http.Error(w, "Function not found", http.StatusNotFound)
        return
    }

    // 4. Delete filesystem entry
    if err := os.Remove(functionPath); err != nil {
        http.Error(w, "Failed to delete file", http.StatusInternalServerError)
        return
    }

    // 5. Delete from service ledger
    if err := service_ledger.DeleteFunctionEntry(fnName); err != nil {
        fmt.Printf("Warning: Failed to delete from ledger: %v\n", err)
        // Continue - file is already deleted
    }

    // 6. Return success response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "message": "Function deleted successfully",
    })
}
```

#### Pattern for ListFunctions

```go
func ListFunctions(w http.ResponseWriter, r *http.Request) {
    // 1. Get filesystem entries
    home, err := os.UserHomeDir()
    functionDir := filepath.Join(home, ".opencloud", "functions")
    files, err := os.ReadDir(functionDir)

    // 2. Get all entries from service ledger
    ledgerFunctions, err := service_ledger.GetAllFunctionEntries()
    if err != nil {
        fmt.Printf("Warning: Failed to read service ledger: %v\n", err)
        ledgerFunctions = make(map[string]service_ledger.FunctionEntry)
    }

    // 3. Merge filesystem and ledger data
    var functions []FunctionItem
    for _, file := range files {
        if file.IsDir() {
            continue
        }

        fn := FunctionItem{
            ID:      file.Name(),
            Name:    file.Name(),
            Runtime: detectRuntime(file.Name()),
            // ... other basic fields
        }

        // 4. Enrich with ledger data if available
        if ledgerEntry, exists := ledgerFunctions[file.Name()]; exists {
            if ledgerEntry.Trigger != "" && ledgerEntry.Schedule != "" {
                fn.Trigger = &Trigger{
                    Type:     ledgerEntry.Trigger,
                    Schedule: ledgerEntry.Schedule,
                    Enabled:  true,
                }
            }
        }

        functions = append(functions, fn)
    }

    // 5. Return list
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(functions)
}
```

### 3. API Route Registration (main.go)

```go
func main() {
    mux := http.NewServeMux()
    
    // Service ledger routes (shared across all services)
    mux.HandleFunc("/get-service-status", service_ledger.GetServiceStatusHandler)
    mux.HandleFunc("/enable-service", service_ledger.EnableServiceHandler)
    
    // Function-specific routes
    mux.HandleFunc("/list-functions", api.ListFunctions)
    mux.HandleFunc("/create-function", api.CreateFunction)
    mux.HandleFunc("/update-function/", api.UpdateFunction)
    mux.HandleFunc("/delete-function", api.DeleteFunction)
    mux.HandleFunc("/invoke-function", api.InvokeFunction)
    mux.HandleFunc("/get-function-logs/", api.GetFunctionLogs)
    
    // ... other routes
    
    handler := withCORS(mux)
    http.ListenAndServe(":3030", handler)
}
```

## Current State of Functions Implementation

### ✅ Already Implemented
1. **Service Ledger Functions** - All CRUD operations exist in `serviceLedger.go`
2. **Backend API Handlers** - Create, Update, Delete, List all use service ledger
3. **Frontend UI** - Service enablement, CRUD operations all exist
4. **API Routes** - All routes registered in `main.go`

### ✅ Current Implementation Analysis

The functions implementation **ALREADY FOLLOWS** the service ledger pattern:

1. **CreateFunction** (`api/compute_handlers.go:413`)
   - ✅ Writes to filesystem
   - ✅ Calls `service_ledger.UpdateFunctionEntry()` (line 493)

2. **UpdateFunction** (`api/compute_handlers.go:513`)
   - ✅ Updates filesystem
   - ✅ Calls `service_ledger.UpdateFunctionEntry()` (line 579)

3. **DeleteFunction** (`api/compute_handlers.go:248`)
   - ✅ Deletes from filesystem
   - ✅ Calls `service_ledger.DeleteFunctionEntry()` (line 275)

4. **ListFunctions** (`api/compute_handlers.go:84`)
   - ✅ Reads from filesystem
   - ✅ Calls `service_ledger.GetAllFunctionEntries()` (line 95)
   - ✅ Merges filesystem and ledger data (lines 126-136)

### 🔍 Key Differences Between Functions and Pipelines

| Aspect | Functions | Pipelines |
|--------|-----------|-----------|
| **Service Name** | "Functions" | "pipelines" |
| **ID Strategy** | Uses filename | Generates unique hex ID |
| **Unique Key** | Function filename (e.g., "hello.py") | Pipeline ID (generated) |
| **Metadata** | Runtime, Trigger, Schedule, Code | ID, Name, Description, Code, Branch, Status, CreatedAt |
| **Sync Feature** | ❌ No sync function | ✅ Has `SyncPipelines()` |

## Best Practices

### 1. Order of Operations
**For CREATE and UPDATE:**
```
1. Validate input
2. Check if exists (both filesystem and ledger)
3. Write to filesystem first
4. Update service ledger second (log but don't fail if ledger update fails)
```

**For DELETE:**
```
1. Validate input
2. Delete from filesystem first
3. Delete from service ledger second (log but don't fail)
```

### 2. Error Handling
- **Filesystem operations are critical** - Fail the request if they fail
- **Ledger operations are important but not critical** - Log warnings if they fail
- The filesystem is the source of truth; ledger provides metadata

### 3. Consistency Checks
- Always check both filesystem AND ledger for existence
- When listing, merge filesystem (source of truth) with ledger (metadata)
- Prefer ledger data when available, fall back to filesystem

### 4. Thread Safety
- All service ledger operations are mutex-protected internally
- No need to implement additional locking in API handlers

## Implementation Checklist for New Services

When implementing a new service with service ledger support:

- [ ] Define entry struct in `service_ledger/serviceLedger.go`
- [ ] Add field to `ServiceStatus` struct
- [ ] Implement CRUD functions in `serviceLedger.go`:
  - [ ] `Update[Service]Entry()`
  - [ ] `Delete[Service]Entry()`
  - [ ] `Get[Service]Entry()`
  - [ ] `GetAll[Service]Entries()`
- [ ] Implement API handlers in `api/` directory:
  - [ ] `Create[Service]()`
  - [ ] `Update[Service]()`
  - [ ] `Delete[Service]()`
  - [ ] `List[Service]()`
  - [ ] `Get[Service]()`
- [ ] Register routes in `main.go`
- [ ] Implement UI in `ui/app/` with:
  - [ ] Service enablement check
  - [ ] Enable service button
  - [ ] CRUD operations
  - [ ] List/search functionality
- [ ] Use service ledger in all CRUD operations
- [ ] Handle errors appropriately (critical for filesystem, warnings for ledger)

## Conclusion

The service ledger pattern provides a consistent way to:
1. **Track service enablement** - Know which services are active
2. **Store metadata** - Keep additional info beyond filesystem
3. **Maintain consistency** - Single source of truth for service state
4. **Enable features** - Support features like triggers, schedules, status tracking

**The functions implementation already follows this pattern correctly!** All the necessary service ledger calls are in place and working as designed.
