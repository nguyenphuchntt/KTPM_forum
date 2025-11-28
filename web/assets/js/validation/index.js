// Validation module - exports all filters and pipeline
import ValidationPipeline from './pipeline.js';
import SizeFilter from './filters/size_filter.js';
import ExtensionFilter from './filters/extension_filter.js';
import MimeTypeFilter from './filters/mime_filter.js';
import MagicBytesFilter from './filters/magic_bytes_filter.js';

// Factory function to create pre-configured image validation pipeline
export function createImageValidationPipeline() {
    return new ValidationPipeline([
        new SizeFilter(5 * 1024 * 1024), // 5MB
        new ExtensionFilter(['.jpg', '.jpeg', '.png', '.gif', '.webp']),
        new MimeTypeFilter(['image/jpeg', 'image/png', 'image/gif', 'image/webp']),
        new MagicBytesFilter({
            jpeg: [0xFF, 0xD8, 0xFF],
            png: [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A],
            gif: [0x47, 0x49, 0x46, 0x38],
            webp: [0x52, 0x49, 0x46, 0x46]
        })
    ]);
}

export {
    ValidationPipeline,
    SizeFilter,
    ExtensionFilter,
    MimeTypeFilter,
    MagicBytesFilter
};
