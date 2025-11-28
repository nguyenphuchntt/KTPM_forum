// MimeTypeFilter - Validates MIME type
class MimeTypeFilter {
    constructor(allowedTypes) {
        this.allowedTypes = allowedTypes;
    }

    async process(file) {
        const mimeType = file.type;

        if (!mimeType) {
            return {
                valid: false,
                error: 'Không thể xác định loại file'
            };
        }

        if (!this.allowedTypes.includes(mimeType)) {
            return {
                valid: false,
                error: `Loại file '${mimeType}' không được hỗ trợ. Chỉ chấp nhận: ${this.allowedTypes.join(', ')}`
            };
        }

        return {
            valid: true,
            metadata: {
                mimeType: mimeType
            }
        };
    }
}

export default MimeTypeFilter;
