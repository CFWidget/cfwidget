package curseforge

func GetReleaseType(i int) string {
	switch i {
	case 1:
		return "release"
	case 2:
		return "beta"
	case 3:
		return "alpha"
	default:
		return "unknown"
	}
}

func IsAllowedFile(i int) bool {
	switch i {
	case 4:
		fallthrough
	case 10:
		return true
	default:
		return false
	}
}
