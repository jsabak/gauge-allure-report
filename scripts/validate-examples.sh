#!/usr/bin/env sh
# SPDX-License-Identifier: Apache-2.0
set -eu

test -f examples/jenkins/Jenkinsfile
grep -F "allure includeProperties: false, jdk: '', results: [[path: 'reports/allure-results']]" examples/jenkins/Jenkinsfile >/dev/null
grep -F "allureVersion: '3'" docs/jenkins.md >/dev/null
test -f examples/github-actions/gauge.yml
grep -F 'gauge run specs' examples/github-actions/gauge.yml >/dev/null
grep -F 'reports/allure-results' examples/github-actions/gauge.yml >/dev/null
echo "Jenkins and GitHub Actions examples validated."
