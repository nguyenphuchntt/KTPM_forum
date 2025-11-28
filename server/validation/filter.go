package validation

// Filter interface - mỗi filter validate một khía cạnh của file
type Filter interface {
	Execute(ctx *ValidationContext) error
}
