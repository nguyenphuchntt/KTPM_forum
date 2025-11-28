// ExtensionFilter - Validates file extension
class ExtensionFilter {
    constructor(allowedExtensions) {
        this.allowedExtensions = allowedExtensions.map(ext => ext.toLowerCase());
    }

    async process(file) {
        const filename = file.name;
        const lastDot = filename.lastIndexOf('.');

        if (lastDot === -1) {
            return {
                valid: false,
                error: 'File không có phần mở rộng'
            };
        }

        const ext = filename.substring(lastDot).toLowerCase();

        if (!this.allowedExtensions.includes(ext)) {
            return {
                valid: false,
                error: `Phần mở rộng '${ext}' không được hỗ trợ. Chỉ chấp nhận: ${this.allowedExtensions.join(', ')}`
            };
        }

        return {
            valid: true,
            metadata: {
                extension: ext
            }
        };
    }
}

export default ExtensionFilter;
