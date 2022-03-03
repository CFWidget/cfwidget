package main

func coalesce(options ...string) string {
	for _, v := range options {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstOrEmpty(data []string) string {
	return firstOr(data, "")
}

func firstOr(data []string, def string) string {
	if len(data) == 0 {
		return def
	}
	return data[0]
}
