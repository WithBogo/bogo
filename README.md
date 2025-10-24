<div align="center">
  <img src="https://withbogo.dev/assets/boGO%20horizontal.png" alt="boGO Logo" style="max-width: 400px; height: auto; object-fit: contain;" />
</div>

# boGO - Boilerplate of Obviously GOlang

[![Website](https://img.shields.io/badge/Website-WithBogo.dev-00ADD8?style=flat&logo=globe)](https://WithBogo.dev)
[![License](https://img.shields.io/badge/License-BSD%203--Clause-blue.svg)](https://opensource.org/licenses/BSD-3-Clause)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org)

**Transform SQL schemas into production-ready Go microservices with hexagonal architecture.**

Generate complete Go services from your database schema in seconds. Includes REST API, database migrations, Docker setup, and clean architecture.

---

## **Why?**

When you need a Go microservice, the architecture choice should be **obvious**:

- **Obviously correct** - True hexagonal architecture, not just layered naming
- **Obviously clean** - Unified interfaces, complete DTO mapping, comprehensive linting  
- **Obviously testable** - Every layer mockable through proper interface separation
- **Obviously maintainable** - Clear layer boundaries with consolidated adapter files
- **Obviously production-ready** - Docker integration, health checks, audit trails
- **Obviously scalable** - Proper dependency inversion enabling easy technology swaps

**boGO transforms your SQL schemas into enterprise-grade Go microservices that follow the best practices from day one.**

### **Perfect for:**
- **Enterprise applications** requiring clean architecture
- **Rapid prototyping** with production-quality code
- **Learning hexagonal architecture** with real examples
- **Legacy modernization** with proper patterns
- **Scalable microservices** built right from the start

---

## **Quick Start**

### Prerequisites
- Go 1.22+
- PostgreSQL (or use Docker)
- SQL schema file

## **1. Generate Code**

```bash
# Clone boGO
git clone https://github.com/WithBogo/boGO
cd boGO

# Generate service from SQL schema
go run . <service-name> <schema-file.sql>

# Example: Generate user service
go run . user-service user_schema.sql
```

**What gets generated:**
- Complete Go microservice with hexagonal architecture
- REST API with full CRUD operations
- Database migrations
- Docker setup with PostgreSQL
- Build scripts and configuration

### Example SQL Schema

```sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

## **2. Setup and Database Migration**

```bash
cd user-service

# Option A: Using Docker (Recommended)
docker-compose up -d  # Starts PostgreSQL and runs migrations automatically

# Option B: Manual Setup
# 1. Create PostgreSQL database
createdb your_database

# 2. Set environment variables
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=your_database
export DB_USER=your_user
export DB_PASSWORD=your_password

# 3. Run migrations with Goose
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -dir migrations postgres "postgres://user:password@localhost/dbname?sslmode=disable" up
```

## **3. Run the Service**

```bash
# Option A: Using Docker
docker-compose up

# Option B: Local development
go run ./cmd/user-service

# Option C: Build and run
./script/build.sh
./build/user-service
```

**Your service will be available at:**
- **API**: `http://localhost:8080`
- **Health Check**: `http://localhost:8080/health`
- **Endpoints**: Auto-generated based on your schema

## **Project Structure**

```
your-service/
├── cmd/your-service/           # Main application
├── config/rest/               # JSON configuration files
├── internal/
│   ├── application/            # Business logic & DTOs
│   ├── domain/model/          # Domain entities
│   ├── interactor/            # REST handlers & adapters
│   └── repository/            # Database implementations
├── migrations/                # Database migrations
├── script/                   # Build scripts
├── docker-compose.yml        # Docker setup
└── Dockerfile               # Container definition
```

## **What You Get**

- **REST API**: Complete CRUD operations for all tables
- **JSON Configuration**: Runtime-configurable query parameters and sorting
- **Database**: PostgreSQL with GORM, migrations with Goose
- **Architecture**: Clean hexagonal architecture with dependency inversion
- **Validation**: Request validation and error handling
- **Docker**: Complete containerization with PostgreSQL
- **Logging**: Structured logging with health checks
- **Testing**: Mockable interfaces for unit testing

## **API Endpoints**

For each table in your schema, boGO generates:

```
GET    /tablename       # List all records (with pagination)
POST   /tablename       # Create new record
GET    /tablename/:id   # Get record by ID
PUT    /tablename/:id   # Update record by ID
DELETE /tablename/:id   # Delete record by ID
GET    /health          # Health check endpoint
```

## **JSON Configuration System**

boGO uses a powerful JSON-based configuration system for REST API query parameters, providing runtime flexibility without recompilation.

### **Configuration Workflow**

```
SQL Schema → Code Generation → JSON Config Files → Runtime Loading
```

When you generate a service, boGO automatically creates JSON configuration files for each entity:

```
your-service/
├── config/rest/                    # JSON Configuration Files
│   ├── user.json                  # Generated from users table
│   ├── product.json               # Generated from products table
│   └── ...
├── internal/interactor/rest/       # REST Layer
│   ├── rest_parameter.go          # Loads from config/rest/
│   ├── user_handler.go            # Uses loaded config
│   └── ...
```

### **Generated JSON Config Example**

For a `users` table, boGO generates `config/rest/user.json`:

```json
{
  "filter": [
    {
      "db_key": "id",
      "query_key": "id",
      "kind": "int64",
      "omitempty": true
    },
    {
      "db_key": "email",
      "query_key": "email",
      "kind": "string",
      "omitempty": true
    },
    {
      "db_key": "name",
      "query_key": "name",
      "kind": "string",
      "omitempty": true
    }
  ],
  "sorting": [
    {
      "db_key": "id",
      "query_key": "id",
      "kind": "int64"
    },
    {
      "db_key": "email",
      "query_key": "email",
      "kind": "string"
    },
    {
      "db_key": "name",
      "query_key": "name",
      "kind": "string"
    }
  ]
}
```

### **Advanced Query Features**

**Filter with multiple parameters:**
```bash
GET /users?name=john&email=@example.com&limit=10&offset=0
```

**Advanced filters:**
```bash
GET /users?name=like(john)&id=range(1,100)&limit=10&offset=0
```
supported command : range(a,b), in(array), like(string), not(string), gte(int), lte(int), gt(int), lt(int), not_in(string)

**Sort by multiple fields:**
```bash
GET /users?sort=asc(name)
```

**Runtime configuration changes:**
```bash
# Edit config file - no restart required for new filters
vim config/rest/user.json

# Add new filterable field
{
  "db_key": "status",
  "query_key": "status",
  "kind": "string",
  "omitempty": true
}
```

### **SQL Type Mapping**

boGO automatically maps SQL types to JSON configuration:

| SQL Type | JSON Kind | Example Query |
|----------|-----------|---------------|
| `BIGINT`, `INTEGER` | `"int64"` | `?user_id=123` |
| `DECIMAL`, `FLOAT` | `"float64"` | `?price=29.99` |
| `BOOLEAN` | `"bool"` | `?active=true` |
| `VARCHAR`, `TEXT` | `"string"` | `?name=john` |

### **Configuration Benefits**

✅ **Runtime Flexibility** - Modify query parameters without recompilation  
✅ **Type Safety** - Automatic type conversion and validation  
✅ **Performance** - Intelligent caching with thread-safe operations  
✅ **Maintainability** - Human-readable JSON format  
✅ **Version Control** - Easy to track configuration changes  
✅ **Environment-Specific** - Different configs for dev/staging/prod  

## **Environment Variables**

```bash
# Database configuration
DB_HOST=localhost
DB_PORT=5432
DB_NAME=your_database
DB_USER=your_user
DB_PASSWORD=your_password

# Server configuration
PORT=8080
```

## **Example Usage**

**Input SQL:**
```sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL
);
```

**Generated API endpoints:**
```bash
# List users with pagination
curl "http://localhost:8080/users?limit=10&offset=0"

# Filter users by email and name (uses JSON config)
curl "http://localhost:8080/users?email=@example.com&name=john"

# Sort users by multiple fields
curl "http://localhost:8080/users?sort=asc(name),desc(id)"

# Create user
curl -X POST http://localhost:8080/users \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","name":"John Doe"}'

# Get user by ID
curl http://localhost:8080/users/1

# Update user
curl -X PUT http://localhost:8080/users/1 \
  -H "Content-Type: application/json" \
  -d '{"email":"updated@example.com","name":"Jane Doe"}'

# Delete user
curl -X DELETE http://localhost:8080/users/1
```

## **Real-World Example**

### Input: Simple SQL Schema
```sql
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

### Output: Complete Microservice
- **Hexagonal Architecture** with 4 layers
- **REST API** with full CRUD operations  
- **JSON Configuration** for runtime query customization
- **PostgreSQL Integration** with migrations
- **Docker Containerization** ready to deploy
- **Testing Framework** with mockable interfaces
- **Monitoring & Logging** with health checks
- **Configuration Management** via environment variables

**From zero to production-ready microservice in seconds!**

*Built with love for the Go community. Making excellent architecture obvious, because life's too short for bad code.*


## **Architectural Stuff**

### **Unified Interface Architecture**
- **Single adapter.go files**: Consolidated interfaces per layer for better maintainability
- **Application Layer**: All repository interfaces in one unified file
- **Interactor Layer**: All service interfaces in one unified file
- **Cleaner imports**: Reduced interface sprawl and improved organization

### **Complete DTO Mapping**
- **Automatic field mapping**: Marshal/Unmarshal methods generated for all entity fields
- **Domain ↔ DTO conversion**: Seamless transformation between layers
- **Type safety**: Full compile-time checking of field mappings

### **Enhanced Response Handling**
- **Consistent API responses**: Response wrapper with proper metadata
- **Smart pagination**: AddMeta implementation with page calculation
- **Error standardization**: Uniform error responses across all endpoints

### **Quality Assurance**
- **Comprehensive linting**: golint integration with automatic installation
- **Code generation gates**: Stops generation if linting issues detected
- **Cross-platform compatibility**: Shell scripts for all major platforms

## **Customization**

The generator uses external templates in the `templates/` directory, making it easy to:

- Modify generated code structure
- Add new features and patterns
- Customize naming conventions
- Extend architecture patterns
- Update response formats and validation rules

---

## **License**

boGO is licensed under the **BSD 3-Clause License**. See [LICENSE](LICENSE) for details.

**What this means:**
- **Commercial use** - Use boGO to generate services for your business
- **Modify and distribute** - Fork, enhance, and share your improvements  
- **Generated code is yours** - No licensing obligations for your microservices
- **Private use** - Use internally without disclosure requirements
- **Attribution required** - Include license notice in redistributions
- **Name protection** - Cannot use "boGO" name to endorse derived products

**boGO - From SQL schema to production-ready Go microservice in seconds.**