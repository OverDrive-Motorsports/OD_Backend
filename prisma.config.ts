import "dotenv/config";
import path from "node:path";
import { defineConfig } from "prisma/config";

const DEFAULT_SCHEMA = "services/auth-service/resources/schema.prisma";
const FALLBACK_DATABASE_URL = "postgresql://prisma:prisma@localhost:5432/overdrive";

const schemaEnvMap: Record<string, string> = {
  "services/auth-service/resources/schema.prisma": "AUTH_DATABASE_URL",
  "services/user-data-service/resources/schema.prisma": "USER_DATA_DATABASE_URL",
  "services/championship-service/resources/schema.prisma": "CHAMPIONSHIP_DATABASE_URL",
  "services/race-data-service/resources/schema.prisma": "RACE_DATA_DATABASE_URL",
};

function getSchemaArg(): string | undefined {
  for (let index = 0; index < process.argv.length; index += 1) {
    const arg = process.argv[index];

    if (arg === "--schema") {
      return process.argv[index + 1];
    }

    if (arg.startsWith("--schema=")) {
      return arg.slice("--schema=".length);
    }
  }

  return undefined;
}

function normalizeSchemaPath(schemaPath: string): string {
  return path.normalize(schemaPath).replace(/\\/g, "/");
}

const requestedSchema = normalizeSchemaPath(getSchemaArg() ?? DEFAULT_SCHEMA);
const schemaEntry =
  Object.entries(schemaEnvMap).find(([schemaPath]) => requestedSchema.endsWith(schemaPath)) ??
  [DEFAULT_SCHEMA, schemaEnvMap[DEFAULT_SCHEMA]];
const [schema, envName] = schemaEntry;
const datasourceUrl = process.env[envName] ?? FALLBACK_DATABASE_URL;

export default defineConfig({
  schema,
  datasource: {
    url: datasourceUrl,
  },
});
