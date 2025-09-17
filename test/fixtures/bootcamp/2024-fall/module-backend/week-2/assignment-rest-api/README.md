# Full-Stack Bootcamp - Backend Module: REST API Development

## Bootcamp Information

- **Program**: 2024 Fall Full-Stack Development Bootcamp
- **Module**: Backend Development
- **Week**: 2
- **Topic**: RESTful API Design and Implementation

## Assignment Overview

Design and implement a complete RESTful API for a task management system with
authentication, CRUD operations, and data persistence.

## Learning Objectives

- Understand REST architectural principles
- Implement HTTP methods and status codes
- Design API endpoints and data models
- Practice authentication and authorization
- Learn database integration and ORM usage

## Requirements

1. Build a Task Management API
2. Implement user authentication (JWT)
3. Create CRUD operations for tasks and projects
4. Add user authorization and permissions
5. Include comprehensive API documentation
6. Implement error handling and validation

## API Endpoints

### Authentication

- POST /api/auth/register - User registration
- POST /api/auth/login - User login
- POST /api/auth/logout - User logout
- GET /api/auth/me - Get current user

### Tasks

- GET /api/tasks - List all tasks (with filtering)
- POST /api/tasks - Create new task
- GET /api/tasks/:id - Get specific task
- PUT /api/tasks/:id - Update task
- DELETE /api/tasks/:id - Delete task

### Projects

- GET /api/projects - List all projects
- POST /api/projects - Create new project
- GET /api/projects/:id - Get project with tasks
- PUT /api/projects/:id - Update project
- DELETE /api/projects/:id - Delete project

## Technical Stack

- **Runtime**: Node.js with Express.js
- **Database**: PostgreSQL with Prisma ORM
- **Authentication**: JWT tokens
- **Validation**: Joi or Zod
- **Testing**: Jest and Supertest
- **Documentation**: Swagger/OpenAPI

## Data Models

### User

- id, email, password, name, createdAt, updatedAt

### Project

- id, title, description, userId, createdAt, updatedAt

### Task

- id, title, description, status, priority, projectId, userId, dueDate,
  createdAt, updatedAt

## Deliverables

- `src/` - Application source code
- `tests/` - API test suite
- `docs/` - API documentation
- `README.md` - Setup and usage instructions
- `postman/` - Postman collection for testing

## Testing Requirements

- Unit tests for all utility functions
- Integration tests for all API endpoints
- Authentication and authorization testing
- Error handling validation
- Achieve 90%+ code coverage

## Grading Rubric

- API functionality: 35%
- Code structure and quality: 25%
- Testing coverage: 20%
- Documentation: 20%
