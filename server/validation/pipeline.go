package validation

import (
	"fmt"
	"io"
)

// Pipeline - Pipe and Filters pattern implementation
// Chạy một chuỗi filters tuần tự
type Pipeline struct {
	filters []Filter
}

// NewPipeline tạo pipeline mới với danh sách filters
func NewPipeline(filters ...Filter) *Pipeline {
	return &Pipeline{filters: filters}
}

// AddFilter thêm filter vào pipeline
func (p *Pipeline) AddFilter(filter Filter) {
	p.filters = append(p.filters, filter)
}

// Execute chạy tất cả filters trên context
// Dừng ngay khi có filter fail
func (p *Pipeline) Execute(ctx *ValidationContext) error {
	for i, filter := range p.filters {
		err := filter.Execute(ctx)
		if err != nil {
			return fmt.Errorf("filter #%d failed: %w", i+1, err)
		}
		
		// Reset file pointer for next filter
		if ctx.Reader != nil {
			ctx.Reader.Seek(0, io.SeekStart)
		}
	}
	
	return nil
}
