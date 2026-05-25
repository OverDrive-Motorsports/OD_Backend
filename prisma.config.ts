import path from "node:path";
import fs from "node:fs";
import { defineConfig } from "prisma/config";

const DEFAULT_SCHEMA = "services/auth-service/resources/schema.prisma";
const ROOT_ENV_PATH = path.resolve(".env");
const schemaConfigMap: Record<string, { envName: string; fallbackUrl: string }> = {
  "services/auth-service/resources/schema.prisma": {
    envName: "AUTH_DATABASE_URL",
    fallbackUrl: "postgresql://postgres:postgres@localhost:5432/overdrive_auth?schema=public",
  },
  "services/user-data-service/resources/schema.prisma": {
    envName: "USER_DATA_DATABASE_URL",
    fallbackUrl: "postgresql://postgres:postgres@localhost:5432/overdrive_user_data?schema=public",
  },
  "services/championship-service/resources/schema.prisma": {
    envName: "CHAMPIONSHIP_DATABASE_URL",
    fallbackUrl: "postgresql://postgres:postgres@localhost:5432/overdrive_championship?schema=public",
  },
  "services/race-data-service/resources/schema.prisma": {
    envName: "RACE_DATA_DATABASE_URL",
    fallbackUrl: "postgresql://postgres:postgres@localhost:5432/overdrive_race_data?schema=public",
  },
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

function loadServiceEnv(schemaPath: string): void {
  const serviceDir = path.resolve(path.dirname(schemaPath), "..");
  const envPath = path.join(serviceDir, ".env");

  loadEnvFile(envPath);
}

function loadEnvFile(envPath: string): void {
  if (!fs.existsSync(envPath)) {
    return;
  }

  const content = fs.readFileSync(envPath, "utf8");
  for (const rawLine of content.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith("#")) {
      continue;
    }

    const separatorIndex = line.indexOf("=");
    if (separatorIndex === -1) {
      continue;
    }

    const key = line.slice(0, separatorIndex).trim();
    const value = line.slice(separatorIndex + 1).trim().replace(/^['"]|['"]$/g, "");
    if (key && process.env[key] === undefined) {
      process.env[key] = value;
    }
  }
}

loadEnvFile(ROOT_ENV_PATH);

const requestedSchema = normalizeSchemaPath(getSchemaArg() ?? DEFAULT_SCHEMA);
const schemaEntry =
  Object.entries(schemaConfigMap).find(([schemaPath]) => requestedSchema.endsWith(schemaPath)) ??
  [DEFAULT_SCHEMA, schemaConfigMap[DEFAULT_SCHEMA]];
const [schema, schemaConfig] = schemaEntry;

loadServiceEnv(schema);

const datasourceUrl = process.env[schemaConfig.envName] ?? schemaConfig.fallbackUrl;

export default defineConfig({
  schema,
  datasource: {
    url: datasourceUrl,
  },
});
