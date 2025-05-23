package util

import "fmt"

func AnyConvertToString(a any) string {
	if a != nil {
		return fmt.Sprintf("%v", a)
	}
	return ""
}
