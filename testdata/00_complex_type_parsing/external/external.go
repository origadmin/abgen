package external

import "time"

type User struct {
	ID        int64
	FirstName string
	LastName  string
	Email     string
	CreatedAt time.Time
	Status    Status
}

type Status int32

const (
	Status_UNKNOWN Status = 0
	Status_ACTIVE  Status = 1
	Status_INACTIVE Status = 2
)

type Order struct {
	OrderID   string
	UserID    int64
	Amount    float64
	OrderTime time.Time
}
