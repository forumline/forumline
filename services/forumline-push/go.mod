module github.com/forumline/forumline/services/forumline-push

go 1.26.1

require (
	github.com/SherClockHolmes/webpush-go v1.4.0
	github.com/ThreeDotsLabs/watermill v1.5.1
	github.com/forumline/forumline/backend v0.0.0-00010101000000-000000000000
	github.com/jackc/pgx/v5 v5.8.0
)

require (
	github.com/ThreeDotsLabs/watermill-nats/v2 v2.1.3 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.18.4 // indirect
	github.com/lithammer/shortuuid/v3 v3.0.7 // indirect
	github.com/nats-io/nats.go v1.49.0 // indirect
	github.com/nats-io/nkeys v0.4.12 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sony/gobreaker v1.0.0 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
)

replace github.com/forumline/forumline/backend => ../../packages/backend
