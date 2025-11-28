// MagicBytesFilter - Validates file signature (magic bytes)
// This is the CRITICAL security filter that checks actual file content
class MagicBytesFilter {
    constructor(signatures) {
        // signatures: {jpeg: [0xFF, 0xD8, 0xFF], png: [...], ...}
        this.signatures = signatures;
    }

    async process(file) {
        return new Promise((resolve) => {
            const reader = new FileReader();

            reader.onload = (e) => {
                const arrayBuffer = e.target.result;
                const bytes = new Uint8Array(arrayBuffer);

                // Check against each known signature
                for (const [fileType, signature] of Object.entries(this.signatures)) {
                    if (this.matchesSignature(bytes, signature)) {
                        resolve({
                            valid: true,
                            metadata: {
                                detectedType: fileType,
                                magicBytesChecked: true
                            }
                        });
                        return;
                    }
                }

                // No signature matched - likely a fake/malicious file
                const bytesHex = Array.from(bytes.slice(0, 8))
                    .map(b => b.toString(16).padStart(2, '0'))
                    .join(' ');

                resolve({
                    valid: false,
                    error: `File không phải ảnh thật! Signature không hợp lệ: ${bytesHex}`
                });
            };

            reader.onerror = () => {
                resolve({
                    valid: false,
                    error: 'Không thể đọc file để kiểm tra signature'
                });
            };

            // Read only first 512 bytes (efficient!)
            const blob = file.slice(0, 512);
            reader.readAsArrayBuffer(blob);
        });
    }

    matchesSignature(bytes, signature) {
        if (bytes.length < signature.length) {
            return false;
        }

        for (let i = 0; i < signature.length; i++) {
            if (bytes[i] !== signature[i]) {
                return false;
            }
        }

        return true;
    }
}

export default MagicBytesFilter;
