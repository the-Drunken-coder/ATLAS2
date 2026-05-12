import fs from "fs";
import path from "path";
import {
  AtlasProtocolValidator,
  type InvalidCaseManifest,
  type ValidExampleManifest,
  normalizeValidationIssues,
} from "../packages/typescript/src";

function main(): void {
  const atlasProtocolRoot = process.env.ATLAS_PROTOCOL_ROOT
    ? path.resolve(process.env.ATLAS_PROTOCOL_ROOT)
    : path.resolve(__dirname, "..", "..");
  const repoRoot = path.resolve(atlasProtocolRoot, "..");
  const validator = new AtlasProtocolValidator(repoRoot, atlasProtocolRoot);

  checkExampleJsonSyntax(validator);
  verifyValidExamples(validator);
  verifyInvalidCases(validator);

  console.log("Atlas Protocol verification passed.");
}

function checkExampleJsonSyntax(validator: AtlasProtocolValidator): void {
  for (const filePath of validator.listExampleFiles()) {
    JSON.parse(fs.readFileSync(filePath, "utf8"));
  }
}

function verifyValidExamples(validator: AtlasProtocolValidator): void {
  const manifest = validator.readManifest<ValidExampleManifest>("source/manifests/valid-examples.json");
  for (const testCase of manifest.cases) {
    const absolutePath = validator.resolveAtlasProtocolPath(testCase.source);
    const document = JSON.parse(fs.readFileSync(absolutePath, "utf8")) as Record<string, unknown>;
    const payload = document[testCase.example];
    if (payload === undefined) {
      throw new Error(
        `valid example ${testCase.id} is missing example key ${testCase.example}`,
      );
    }
    const issues = validator.validateValue(testCase.resource, payload, {
      variant: testCase.variant,
    });
    if (issues.length > 0) {
      throw new Error(
        `valid example ${testCase.id} failed validation: ${JSON.stringify(issues)}`,
      );
    }
  }
}

function verifyInvalidCases(validator: AtlasProtocolValidator): void {
  const manifest = validator.readManifest<InvalidCaseManifest>("source/manifests/invalid-cases.json");
  for (const testCase of manifest.cases) {
    testCase.expected.forEach((issue) => validator.assertValidationIssue(issue));

    const absolutePath = validator.resolveAtlasProtocolPath(testCase.source);
    const text = fs.readFileSync(absolutePath, "utf8");
    const issues = validator.validateText(testCase.resource, text, testCase.variant);

    if (issues.length === 0) {
      throw new Error(`invalid case ${testCase.id} unexpectedly passed validation`);
    }
    issues.forEach((issue) => validator.assertValidationIssue(issue));

    const actual = normalizeValidationIssues(issues);
    const expected = normalizeValidationIssues(testCase.expected);
    if (actual.length !== expected.length) {
      throw new Error(
        `invalid case ${testCase.id}: expected ${expected.length} issues, got ${actual.length}\nexpected=${JSON.stringify(expected)}\nactual=${JSON.stringify(actual)}`,
      );
    }
    for (let i = 0; i < actual.length; i += 1) {
      const a = actual[i];
      const e = expected[i];
      if (a.field !== e.field || a.code !== e.code || a.message !== e.message) {
        throw new Error(
          `invalid case ${testCase.id}: mismatch at index ${i}\nexpected=${JSON.stringify(e)}\nactual=${JSON.stringify(a)}\nfullActual=${JSON.stringify(actual)}`,
        );
      }
    }
  }
}

main();
