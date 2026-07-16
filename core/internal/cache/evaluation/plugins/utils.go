package plugins

// clamp 限制数值范围
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// ptrFloat64 返回float64指针
func ptrFloat64(f float64) *float64 {
	return &f
}
