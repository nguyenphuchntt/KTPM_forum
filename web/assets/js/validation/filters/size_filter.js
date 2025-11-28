// SizeFilter - Validates file size
class SizeFilter {
    constructor(maxSize) {
        this.maxSize = maxSize;
        this.maxSizeMB = (maxSize / (1024 * 1024)).toFixed(2);
    }

    async process(file) {
        if (file.size > this.maxSize) {
            const fileSizeMB = (file.size / (1024 * 1024)).toFixed(2);
            return {
                valid: false,
                error: `File quá lớn: ${fileSizeMB}MB vượt quá giới hạn ${this.maxSizeMB}MB`
            };
        }

        return {
            valid: true,
            metadata: {
                sizeChecked: true,
                fileSize: file.size
            }
        };
    }
}

export default SizeFilter;
