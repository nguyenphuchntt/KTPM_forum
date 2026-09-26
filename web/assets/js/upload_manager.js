const maxAttempts = 4;

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

            // Phase 2: Ask the server for a pre-signed PUT to the quarantine bucket
            this.updateProgress(30, 'Đang yêu cầu quyền upload...');
            const ticket = await this.requestUploadURL(file);

            // Phase 3: Upload straight to object storage
            this.updateProgress(50, 'Đang upload lên cloud...');
            await this.uploadToStorage(file, ticket.upload_url);

            // Phase 4: Let the server validate and publish the object. This is
            // synchronous, so once it answers the image is already live — there
            // is nothing left for the client to wait for.
            this.updateProgress(90, 'Đang xác thực ảnh...');
            const media = await this.confirmUpload(ticket.object_key);

            this.updateProgress(100, 'Upload thành công!');

            return {
                success: true,
                mediaId: media.media_id,
                imageURL: media.public_url,
                objectKey: ticket.object_key
            };

        } catch (error) {
            console.error('[UploadManager] Upload failed:', error);
            throw error;
        }
    }

    async requestUploadURL(file) {
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
            const error = await response.json().catch(() => ({}));
            throw new Error(this.errorMessage(response, error, 'Không thể lấy quyền upload'));
        }

        return response.json();
    }

    async confirmUpload(objectKey) {
        const response = await fetch('/api/upload/confirm', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ object_key: objectKey })
        });

        if (!response.ok) {
            const error = await response.json().catch(() => ({}));
            throw new Error(this.errorMessage(response, error, 'Ảnh không hợp lệ'));
        }

        return response.json();
    }

    // Errors normally come back in the shared envelope
    // {"error": {code, message, details}}, but the endpoint rate limiter answers
    // 429 with an empty body, so the status is the fallback signal.
    errorMessage(response, body, fallback) {
        if (body?.error?.message) {
            return body.error.message;
        }
        if (response.status === 429) {
            return 'Bạn thao tác quá nhanh, vui lòng thử lại sau.';
        }
        return fallback;
    }

    sleep(time) {
        return new Promise(resolve => setTimeout(resolve, time));
    }

    // The pre-signed URL already carries the signature in its query string, so
    // the request must not add authentication headers of its own.
    async uploadToStorage(file, uploadURL) {
        let lastError;
        for (let i = 0; i < maxAttempts; i++) {
            try {
                const response = await fetch(uploadURL, {
                    method: 'PUT',
                    headers: {
                        'Content-Type': file.type
                    },
                    body: file
                });
                if (response.ok) {
                    return;
                } else {
                    throw new Error(`Failed to upload ${response.status}`);
                }
            } catch (error) {
                lastError = error;
                if (i < maxAttempts - 1) {
                    await this.sleep(1000);
                }
            }
        }
        throw new Error(`Failed to upload after ${maxAttempts} attempts: ${lastError.message}`);
    }
}

export default UploadManager;
