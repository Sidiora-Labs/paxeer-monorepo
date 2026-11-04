import * as fs from 'fs';
import * as path from 'path';
import { parse, stringify } from 'yaml';
import { mergeWith } from 'lodash';

const OVERRIDES_DIR = path.join(__dirname, '..', 'swaggers', 'overrides');
const MAIN_SPEC_PATH = path.join(__dirname, '..', 'swaggers', 'blockscout-api-v2.yaml');
const OUTPUT_PATH = path.join(__dirname, '..', 'swaggers', 'blockscout-api-final.yaml');

type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue };

function replaceArrays(_target: JsonValue, source: JsonValue): JsonValue | undefined {
    if (Array.isArray(source)) {
        return source.slice();
    }
    return undefined;
}

function validateRequiredProperties(schema: JsonValue, location: string): void {
    if (Array.isArray(schema)) {
        schema.forEach((item, index) => validateRequiredProperties(item, `${location}/${index}`));
        return;
    }
    if (!schema || typeof schema !== 'object') {
        return;
    }
    const required = schema.required;
    if (required !== undefined) {
        if (!Array.isArray(required) || required.length === 0 ||
            required.some(name => typeof name !== 'string') ||
            new Set(required).size !== required.length) {
            throw new Error(`Invalid required property list at ${location}`);
        }
        const properties = schema.properties;
        if (properties && typeof properties === 'object' && !Array.isArray(properties)) {
            for (const name of required) {
                if (typeof name === 'string' && !Object.prototype.hasOwnProperty.call(properties, name)) {
                    throw new Error(`Unknown required property ${name} at ${location}`);
                }
            }
        }
    }
    const properties = schema.properties;
    if (properties && typeof properties === 'object' && !Array.isArray(properties)) {
        for (const [name, property] of Object.entries(properties)) {
            validateRequiredProperties(property, `${location}/properties/${name}`);
        }
    }
    for (const keyword of ['items', 'additionalProperties', 'allOf', 'anyOf', 'oneOf', 'not']) {
        if (schema[keyword] !== undefined) {
            validateRequiredProperties(schema[keyword], `${location}/${keyword}`);
        }
    }
}

function removeMarkedFields(obj: JsonValue): JsonValue {
    if (Array.isArray(obj)) {
        return obj.map(item => removeMarkedFields(item));
    }
    if (obj && typeof obj === 'object') {
        const result: { [key: string]: JsonValue } = {};
        for (const [key, value] of Object.entries(obj)) {
            if (value === '__REMOVE__') {
                continue; // Skip this field
            }
            result[key] = removeMarkedFields(value);
        }
        return result;
    }
    return obj;
}

async function mergeOverrides() {
    try {
        // Read the main spec file
        const mainSpecContent = fs.readFileSync(MAIN_SPEC_PATH, 'utf8');
        let mergedSpec = parse(mainSpecContent);

        // Read all YAML files from the overrides directory
        const overrideFiles = fs.readdirSync(OVERRIDES_DIR)
            .filter(file => file.endsWith('.yaml'));

        // Sort files to ensure consistent merging order
        overrideFiles.sort();

        // Apply each override
        for (const file of overrideFiles) {
            const overridePath = path.join(OVERRIDES_DIR, file);
            const overrideContent = fs.readFileSync(overridePath, 'utf8');
            const overrideSpec = parse(overrideContent);

            // Replace arrays completely; merging by index retains upstream required fields.
            mergedSpec = mergeWith(mergedSpec, overrideSpec, replaceArrays);
        }

        // Remove marked fields
        mergedSpec = removeMarkedFields(mergedSpec);

        for (const [name, schema] of Object.entries(mergedSpec.components.schemas)) {
            validateRequiredProperties(schema as JsonValue, `#/components/schemas/${name}`);
        }

        // Write the merged result back to the main spec file
        const mergedYaml = stringify(mergedSpec);
        fs.writeFileSync(OUTPUT_PATH, mergedYaml);

        console.log(`Successfully merged all overrides into ${OUTPUT_PATH}`);
    } catch (error) {
        console.error('Error merging overrides:', error);
        process.exit(1);
    }
}

mergeOverrides();
