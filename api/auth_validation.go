package api

import "strings"

type registrationValidationInput struct {
	Email       string
	Password    string
	Name        string
	DateOfBirth string
	Age         int
	Weight      float64
	Height      float64
	TDEE        int
}

func validateRegistrationInput(input registrationValidationInput) error {
	errs := NewValidationErrors()

	email := strings.TrimSpace(input.Email)
	if email == "" {
		errs.Add("email", "email is required")
	}

	name := strings.TrimSpace(input.Name)
	if name == "" {
		errs.Add("name", "name is required")
	}

	password := strings.TrimSpace(input.Password)
	switch {
	case password == "":
		errs.Add("password", "password is required")
	case len(password) < minPasswordLength:
		errs.Add("password", "password must be at least 8 characters")
	}

	if rawDate := strings.TrimSpace(input.DateOfBirth); rawDate != "" {
		if _, err := parseOptionalBirthDate(rawDate); err != nil {
			errs.Add("date_of_birth", err.Error())
		}
	}

	if input.Age < 0 {
		errs.Add("age", "age cannot be negative")
	}
	if input.Weight < 0 {
		errs.Add("weight", "weight cannot be negative")
	}
	if input.Height < 0 {
		errs.Add("height", "height cannot be negative")
	}
	if input.TDEE < 0 {
		errs.Add("tdee", "tdee cannot be negative")
	}

	if errs.Any() {
		return errs
	}
	return nil
}

func validateCredentialFields(email, password string) error {
	errs := NewValidationErrors()

	if strings.TrimSpace(email) == "" {
		errs.Add("email", "email is required")
	}

	trimmedPassword := strings.TrimSpace(password)
	switch {
	case trimmedPassword == "":
		errs.Add("password", "password is required")
	case len(trimmedPassword) < minPasswordLength:
		errs.Add("password", "password must be at least 8 characters")
	}

	if errs.Any() {
		return errs
	}
	return nil
}
