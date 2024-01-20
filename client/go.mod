module github.com/P4nos/tcp-tunnel/client

go 1.20

require (
	github.com/P4nos/tcp-tunnel/common v0.0.0
	github.com/joho/godotenv v1.5.1
)

replace github.com/P4nos/tcp-tunnel/common v0.0.0 => ../common
