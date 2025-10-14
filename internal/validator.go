package internal

import (
	//"NYCU-SDC/core-system-backend/internal/tenant"
	//"NYCU-SDC/core-system-backend/internal/unit"
	//"context"
	"github.com/go-playground/validator/v10"
	//"github.com/google/uuid"
)

//func SlugExistValidator(tenantService tenant.Service) validator.Func {
//	return func(fl validator.FieldLevel) bool {
//		slug := fl.Parent().FieldByName("slug").String()
//		exist, err := tenantService.SlugExists(context.Background(), slug)
//		return err == nil && exist
//	}
//}
//
//func UnitExistValidator(unitService unit.Service) validator.Func {
//	return func(fl validator.FieldLevel) bool {
//		unitID, err := uuid.Parse(fl.Parent().FieldByName("unitID").String())
//		if err != nil {
//			return false // invalid UUID
//		}
//
//		exist, err := unitService.UnitExists(context.Background(), unitID)
//		return err == nil && exist
//	}
//}
//
//func SlugBelongsToUnitValidator(tenantService tenant.Service) validator.Func {
//	return func(fl validator.FieldLevel) bool {
//		slug := fl.Parent().FieldByName("slug").String()
//		unitID := fl.Parent().FieldByName("unitID").String()
//		_, curUnitID, err := tenantService.GetStatus(context.Background(), slug)
//		return err == nil && curUnitID.String() == unitID
//	}
//}

func NewValidator() *validator.Validate {
	v := validator.New()
	//v.RegisterValidation("SlugExist", SlugExistValidator(tenantService))
	//v.RegisterValidation("UnitExist", UnitExistValidator(unitService))
	//v.RegisterValidation("SlugBelongsToUnit", SlugBelongsToUnitValidator(tenantService))
	return v
}

func ValidateStruct(v *validator.Validate, s interface{}) error {
	err := v.Struct(s)
	if err != nil {
		return err
	}
	return nil
}
