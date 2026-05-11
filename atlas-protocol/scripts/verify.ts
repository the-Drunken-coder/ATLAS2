import fs from "fs";
import path from "path";
import {
  AtlasProtocolValidator,
  type InvalidCaseManifest,
  type ValidExampleManifest,
} from "../packages/typescript/src";

function main(): void {
  const atlasProtocolRoot = path.resolve(__dirname, "..", "..");
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
  const manifest = validator.readManifest<ValidExampleManifest>(
    "atlas-protocol/source/manifests/valid-examples.json",
  );
  for (const testCase of manifest.cases) {
    const absolutePath = validator.resolveRepoPath(testCase.source);
    const document = JSON.parse(fs.readFileSync(absolutePath, "utf8")) as Record<
      string,
      unknown
    >;
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
  const manifest = validator.readManifest<InvalidCaseManifest>(
    "atlas-protocol/source/manifests/invalid-cases.json",
  );
  for (const testCase of manifest.cases) {
    testCase.expected.forEach((issue) => validator.assertValidationIssue(issue));

    const absolutePath = validator.resolveRepoPath(testCase.source);
    const text = fs.readFileSync(absolutePath, "utf8");
    const issues = validator.validateText(testCase.resource, text, testCase.variant);

    if (issues.length === 0) {
      throw new Error(`invalid case ${testCase.id} unexpectedly passed validation`);
    }
    issues.forEach((issue) => validator.assertValidationIssue(issue));

    for (const expectedIssue of testCase.expected) {
      const matched = issues.some(
        (issue) =>
          issue.field === expectedIssue.field &&
          issue.code === expectedIssue.code &&
          issue.message === expectedIssue.message,
      );
      if (!matched) {
        throw new Error(
          `invalid case ${testCase.id} did not emit expected issue ${JSON.stringify(expectedIssue)}; actual=${JSON.stringify(issues)}`,
        );
      }
    }
  }
}

main();
