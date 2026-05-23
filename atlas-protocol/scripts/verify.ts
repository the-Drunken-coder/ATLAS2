import fs from "fs";
import {
  AtlasProtocolValidator,
  type InvalidCaseManifest,
  type InvalidHistoryCaseManifest,
  type ValidExampleManifest,
  normalizeValidationIssues,
} from "../packages/typescript/src";
import { resolveAtlasProtocolRuntimeRoot, resolveRepoRoot } from "./path-utils";

function main(): void {
  const atlasProtocolRoot = resolveAtlasProtocolRuntimeRoot();
  const repoRoot = resolveRepoRoot(atlasProtocolRoot);
  const validator = new AtlasProtocolValidator(repoRoot, atlasProtocolRoot);

  checkExampleJsonSyntax(validator);
  verifyValidExamples(validator);
  verifyInvalidCases(validator);
  verifyInvalidHistoryCases(validator);

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

function assertInvalidCaseMatches(
  id: string,
  expectedIssues: { field: string; code: string; message: string }[],
  actualIssues: { field: string; code: string; message: string }[],
  label: "invalid case" | "invalid history case",
): void {
  const actual = normalizeValidationIssues(actualIssues);
  const expected = normalizeValidationIssues(expectedIssues);
  if (actual.length !== expected.length) {
    throw new Error(
      `${label} ${id}: expected ${expected.length} issues, got ${actual.length}\nexpected=${JSON.stringify(expected)}\nactual=${JSON.stringify(actual)}`,
    );
  }
  for (let i = 0; i < actual.length; i += 1) {
    const a = actual[i];
    const e = expected[i];
    if (a.field !== e.field || a.code !== e.code || a.message !== e.message) {
      throw new Error(
        `${label} ${id}: mismatch at index ${i}\nexpected=${JSON.stringify(e)}\nactual=${JSON.stringify(a)}\nfullActual=${JSON.stringify(actual)}`,
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

    assertInvalidCaseMatches(testCase.id, testCase.expected, issues, "invalid case");
  }
}

function verifyInvalidHistoryCases(validator: AtlasProtocolValidator): void {
  const manifest = validator.readManifest<InvalidHistoryCaseManifest>(
    "source/manifests/invalid-history-cases.json",
  );
  for (const testCase of manifest.cases) {
    testCase.expected.forEach((issue) => validator.assertValidationIssue(issue));

    const absolutePath = validator.resolveAtlasProtocolPath(testCase.source);
    const text = fs.readFileSync(absolutePath, "utf8");
    const issues = validator.validateObservationHistoryEvent(text);

    if (issues.length === 0) {
      throw new Error(`invalid history case ${testCase.id} unexpectedly passed validation`);
    }
    issues.forEach((issue) => validator.assertValidationIssue(issue));

    assertInvalidCaseMatches(testCase.id, testCase.expected, issues, "invalid history case");
  }
}

main();
