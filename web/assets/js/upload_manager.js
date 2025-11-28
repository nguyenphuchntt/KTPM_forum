// UploadManager - Orchestrates the complete upload flow
// 1. Validate client-side
// 2. Request SAS token from server
// 3. Upload to Azure quarantine
// 4. Poll validation status
import { createImageValidationPipeline } from './validation/index.js';

class UploadManager {
    constructor() {
        this.pipeline = createImageValidationPipeline();
        this.progressCallback = null;
    }

    setProgressCallback(callback) {
        this.progressCallback = callback;
    }

    updateProgress(percent, message) {
        if (this.progressCallback) {
            this.progressCallback(percent, message);
        }
    }

    async upload(file) {
        try {
            // Phase 1: Client-side validation
            this.updateProgress(10, 'Đang kiểm tra file...');
            await this.pipeline.execute(file);

            // Phase 2: Request SAS token from Gatekeeper
            this.updateProgress(30, 'Đang yêu cầu quyền upload...');
            const sasData = await this.requestSASToken(file);

            // Phase 3: Direct upload to Azure quarantine
            this.updateProgress(50, 'Đang upload lên cloud...');
            await this.uploadToAzure(file, sasData.upload_url);

            // Phase 4: Notify server and get public URL
            this.updateProgress(90, 'Đang hoàn tất...');
            const imageURL = sasData.public_url;

            this.updateProgress(100, 'Upload thành công!');

            return {
                success: true,
                imageURL: imageURL,
                objectKey: sasData.object_key
            };

        } catch (error) {
            console.error('[UploadManager] Upload failed:', error);
            throw error;
        }
    }

    async requestSASToken(file) {
        const response = await fetch('/api/upload/request-url', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                filename: file.name,
                size: file.size,
                content_type: file.type
            })
        });

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.message || 'Không thể lấy quyền upload');
        }

        return response.json();
    }

    async uploadToAzure(file, sasURL) {
        const response = await fetch(sasURL, {
            method: 'PUT',
            headers: {
                'x-ms-blob-type': 'BlockBlob',
                'Content-Type': file.type
            },
            body: file
        });

        if (!response.ok) {
            throw new Error('Upload lên cloud thất bại');
        }
    }
}

export default UploadManager;
