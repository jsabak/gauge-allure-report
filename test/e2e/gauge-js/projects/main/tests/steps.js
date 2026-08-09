/* SPDX-License-Identifier: Apache-2.0 */
/* globals gauge, step, beforeScenario, afterScenario */

"use strict";

const assert = require("node:assert");

beforeScenario(function () {
  gauge.message("before JavaScript scenario");
});

afterScenario(function () {
  gauge.message("after JavaScript scenario");
});

step("JavaScript passes with <value>", function (value) {
  gauge.message("JavaScript runner message");
  assert.equal(value, "portable");
});
