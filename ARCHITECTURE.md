# Storage Service Architecture

## Fx Module Dependency Diagram

```mermaid
graph TD
    %% Configuration Layer
    CLI[CLI Command<br/>cmd/cli/serve/ucan.go] --> AppConfig[app.Config<br/>pkg/config/app]
    
    %% Core Dependencies
    AppConfig --> ConfigModule[servicesconfig.Module<br/>pkg/services/config]
    AppConfig --> DatastoreModule[datastores.FilesystemModule<br/>pkg/datastores]
    
    %% What ConfigModule Provides
    ConfigModule --> |provides| Principal[principal.Signer]
    ConfigModule --> |provides| Connections[Service Connections<br/>- Upload Service<br/>- Indexing Service]
    ConfigModule --> |provides| URLs[Service URLs/DIDs]
    ConfigModule --> |provides| PDPService[PDP Service<br/>Optional]
    ConfigModule --> |provides| Presigner[URL Presigner]
    ConfigModule --> |provides| Access[Access Control]
    
    %% What DatastoreModule Provides
    DatastoreModule --> |provides| Stores[Data Stores<br/>- BlobStore<br/>- AllocationStore<br/>- ClaimStore<br/>- PublisherStore<br/>- ReceiptStore]
    
    %% Service Layer
    Stores --> ServiceModule[services.ServiceModule<br/>pkg/services]
    Connections --> ServiceModule
    Principal --> ServiceModule
    PDPService --> ServiceModule
    
    %% Service Implementations
    ServiceModule --> BlobService[blob.ServiceModule]
    ServiceModule --> ClaimService[claim.ServiceModule]
    ServiceModule --> PublisherService[publisher.ServiceModule]
    ServiceModule --> StorageService[storage.ServiceModule]
    ServiceModule --> ReplicatorService[replicator.ServiceModule]
    
    %% HTTP Layer
    BlobService --> HTTPHandlers[services.HTTPHandlersModule]
    ClaimService --> HTTPHandlers
    PublisherService --> HTTPHandlers
    
    %% UCAN Layer
    BlobService --> UCANMethods[services.UCANMethodsModule]
    StorageService --> UCANMethods
    ReplicatorService --> UCANMethods
    PDPService --> UCANMethods
    
    %% Server Layer
    HTTPHandlers --> Server[server.Module<br/>Echo HTTP Server]
    UCANMethods --> Server
    
    %% Styling
    classDef config fill:#e1f5fe,stroke:#01579b,stroke-width:2px
    classDef datastore fill:#f3e5f5,stroke:#4a148c,stroke-width:2px
    classDef service fill:#e8f5e9,stroke:#1b5e20,stroke-width:2px
    classDef http fill:#fff3e0,stroke:#e65100,stroke-width:2px
    classDef server fill:#fce4ec,stroke:#880e4f,stroke-width:2px
    
    class AppConfig,ConfigModule,URLs,Connections,Principal,PDPService,Presigner,Access config
    class DatastoreModule,Stores datastore
    class ServiceModule,BlobService,ClaimService,PublisherService,StorageService,ReplicatorService service
    class HTTPHandlers,UCANMethods http
    class Server server
```

## Module Breakdown

### 1. **Configuration Layer**
- **Input**: `app.Config` (transformed from CLI flags/config file)
- **Modules**:
  - `servicesconfig.Module` - Provides configuration-derived dependencies
  - `datastores.FilesystemModule` - Provides data storage implementations

### 2. **Core Dependencies** (What gets provided)
From `servicesconfig.Module`:
- `principal.Signer` - Service identity
- Service connections (Upload, Indexing)
- Service URLs and DIDs
- PDP Service (optional)
- URL presigner for blob access
- Access control patterns

From `datastores.FilesystemModule`:
- `BlobStore` - Binary data storage
- `AllocationStore` - Space allocations
- `ClaimStore` - UCAN delegations
- `PublisherStore` - IPNI publishing state
- `ReceiptStore` - Task receipts

### 3. **Service Layer**
`services.ServiceModule` contains:
- `blob.ServiceModule` - Blob storage logic
- `claim.ServiceModule` - Claim/delegation management
- `publisher.ServiceModule` - IPNI publishing
- `storage.ServiceModule` - Storage orchestration
- `replicator.ServiceModule` - Replication logic

### 4. **API Layer**
- `services.HTTPHandlersModule` - REST/HTTP endpoints
- `services.UCANMethodsModule` - UCAN-based RPC handlers

### 5. **Server Layer**
- `server.Module` - Echo HTTP server with middleware

## Dependency Flow

1. **Config Transformation**: CLI flags → `app.Config`
2. **Dependency Creation**: `app.Config` → Config providers + Datastore providers
3. **Service Creation**: Dependencies → Service implementations
4. **API Registration**: Services → HTTP/UCAN handlers
5. **Server Start**: Handlers → Echo server

## Key Design Principles

1. **Separation of Concerns**: Each module has a single responsibility
2. **Explicit Dependencies**: No hidden dependencies between modules
3. **Layered Architecture**: Clear layers from config → storage → services → API
4. **Testability**: Each layer can be tested independently
5. **Flexibility**: Modules can be swapped (e.g., filesystem → AWS datastores)