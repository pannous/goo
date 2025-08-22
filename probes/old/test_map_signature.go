package main; import "slices"; func main() { nums := []int{1,2}; _ = slices.Map(nums, func(x int) string { return "" }) }
