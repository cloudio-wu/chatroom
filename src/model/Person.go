package models

type PersonID int

type Person struct {
	Id   PersonID `json:"id"`
	Name string   `json:name`
}
