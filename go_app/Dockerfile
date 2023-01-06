FROM golang:latest

# Set the current working directory inside the container
WORKDIR /app

# Copy the go.mod and go.sum files to the working directory
COPY go.mod go.sum ./

# Download all the dependencies
RUN go mod download

# Copy the source code to the working directory
COPY . .

# Build the Go application for Linux
RUN GOOS=linux GOARCH=amd64 go build -o main .

# Expose the port 8000 to the outside world
EXPOSE 9000

# Run the Go application
CMD ["./main"]
