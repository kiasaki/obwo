# AGENTS.md - obwo Cloud Platform

## Overview

This document provides guidance for AI coding agents working on the **obwo** cloud platform. obwo is a personal cloud implementation built in Go, featuring microservices architecture with distributed database capabilities.

## Project Architecture

### High-Level Structure
```
obwo/
├── apps/           # Microservice applications
├── libraries/      # Shared libraries and utilities  
├── tools/         # Database and utility tools
└── makefile       # Build and deployment scripts
```

### Core Components

#### Applications (`/apps/`)
- **auth** (port 9001) - Authentication service handling login, registration, and settings
- **search** (port 9002) - Search service with Brave API integration

#### Libraries (`/libraries/`)
- **app** - Core application framework with HTTP server, context management, and middleware
- **log** - Logging utilities
- **static** - Shared static assets (CSS, JS, fonts)
- **util** - Common utilities (crypto, database, HTTP, validation, conversions)

#### Tools (`/tools/`)
- **db** - Distributed database with Raft consensus
- **dbcli** - Database command-line interface  
- **load** - Load testing utilities
- **migrate** - Database migration tool
- **run** - Application runner

## Development Guidelines

### Technology Stack
- **Language**: Go 1.24.0
- **Database**: SQLite with modernc.org/sqlite driver
- **Consensus**: Raft protocol implementation
- **Templates**: Go html/template
- **Crypto**: golang.org/x/crypto

### Code Patterns

#### Application Structure
Each microservice follows this pattern:
```go
func main() {
    server := app.NewServer("service-name")
    server.Port = 9001
    server.Templates(templatesFS)
    server.Static(staticFS)
    server.Handle("/{$}", handleRoot)
    server.Start()
}
```

#### Handler Pattern
```go
func handleExample(c *app.Context) {
    // Authentication check
    user := c.CurrentUser()
    
    // Form processing
    if c.Req.Method == "POST" {
        // Validation
        c.Errors = util.Validate(c.Params, validators...)
        if len(c.Errors) > 0 {
            goto render
        }
        // Process and redirect
    }
render:
    c.Render(200, "template", util.J{"data": value})
}
```

#### Database Operations
```go
// Query
users := c.SQL("select id, username from users where id = ?", userID)

// Insert/Update
c.SQL("insert into table (col1, col2) values (?, ?)", val1, val2)
```

### Authentication System
- Cookie-based sessions using JWT tokens
- Password hashing with bcrypt
- Cross-service auth via `?auth=token` parameter
- User context available via `c.CurrentUser()`

### Database Architecture
- Distributed SQLite with Raft consensus
- Three-node cluster by default (:8100, :8101, :8102)
- WAL mode for performance
- HTTP API for SQL operations

### Building and Running

#### Development
```bash
make build          # Build all services
make run           # Run auth + search + load
make rundb         # Run 3-node database cluster
make migrate       # Run database migrations
```

#### Database Testing
```bash
make testdb        # Test database operations
make db           # Open database CLI
```

### Planned Services Architecture

The project roadmap includes these additional services:

#### Core Services
- **mail** - Email management (read, send, organize)
- **calendar** - Calendar views, creation, import
- **contacts** - Contact management
- **tasks** - Task list management
- **drive** - File storage and management
- **docs** - Document editing (documents, spreadsheets, slides)

#### Content Services  
- **notes** - Note-taking application
- **photos** - Photo management with thumbnails and EXIF
- **news** - RSS feed aggregation and sanitization
- **maps** - OpenStreetMap rendering

#### Utility Services
- **search tools** - Weather, calculator, unit conversions, password generator

### Code Quality Guidelines

#### Error Handling
- Use `util.Check(err)` for critical errors
- Use `c.Errors` slice for user-facing validation errors
- Implement panic recovery in HTTP handlers

#### Validation
```go
c.Errors = util.Validate(c.Params,
    util.ValidateRegexp("field", regexp.MustCompile("pattern")),
    util.ValidateUnique("field", "table", "column", "exclude"),
    util.ValidateLength("field", minLen, maxLen),
)
```

#### Security
- Use `util.CreatePassword()` for password hashing
- Use `util.CheckPassword()` for verification
- Use `util.CreateToken()` for JWT generation
- Validate all user inputs

### Common Patterns

#### Service Communication
Services communicate via HTTP redirects with auth tokens:
```go
c.Redirect(c.App("service-name") + "/?auth=" + token)
```

#### Static Assets
- Shared assets copied to each service during build
- Service-specific assets in `/static/` directory
- Templates in `/templates/` directory using embed.FS

#### Environment Configuration
```go
port := util.Env("PORT", "default-value")
secret := util.Env("SECRET", "default-secret")
```

### Working with Agents

When making changes:

1. **Read existing code** - Understand patterns before implementing
2. **Follow conventions** - Maintain consistency with existing services
3. **Test thoroughly** - Use make commands to verify functionality
4. **Handle errors gracefully** - Use established error patterns
5. **Update documentation** - Keep README and comments current

### Database Schema Patterns

Tables should include standard fields:
```sql
CREATE TABLE example (
    id INTEGER PRIMARY KEY,
    created INTEGER NOT NULL,  -- Unix timestamp
    updated INTEGER NOT NULL,  -- Unix timestamp
    -- other fields
);
```

### File Organization
- Keep services independent in `/apps/`
- Share common code via `/libraries/`
- Use embedded file systems for templates and static assets
- Tools should be self-contained in `/tools/`

This architecture supports horizontal scaling, service independence, and maintains simplicity while providing enterprise-grade features.