// ValidationPipeline - Pipe and Filters Pattern implementation
// Executes a series of filters sequentially on a file
class ValidationPipeline {
    constructor(filters) {
        this.filters = filters;
    }

    async execute(file) {
        console.log(`[Pipeline] Starting validation for: ${file.name}`);

        for (let i = 0; i < this.filters.length; i++) {
            const filter = this.filters[i];
            const filterName = filter.constructor.name;

            console.log(`[Pipeline] Filter ${i + 1}/${this.filters.length}: ${filterName}`);

            const result = await filter.process(file);

            if (!result.valid) {
                console.error(`[Pipeline] ✗ ${filterName} failed:`, result.error);
                throw new Error(result.error);
            }

            console.log(`[Pipeline] ✓ ${filterName} passed`);

            // Store metadata for potential use by next filters
            if (result.metadata) {
                file._validationMetadata = file._validationMetadata || {};
                Object.assign(file._validationMetadata, result.metadata);
            }
        }

        console.log(`[Pipeline] ✓ All ${this.filters.length} filters passed`);
        return true;
    }
}

export default ValidationPipeline;
