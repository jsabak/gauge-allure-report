# Jenkins

Install the Jenkins Allure plugin and make an Allure command-line installation available through Jenkins global tools. Always publish in `post { always { ... } }` so failed Gauge builds retain diagnostics.

## Allure 2

```groovy
pipeline {
  agent any

  stages {
    stage('Gauge') {
      steps {
        sh 'gauge run specs'
      }
    }
  }

  post {
    always {
      allure includeProperties: false,
             jdk: '',
             results: [[path: 'reports/allure-results']]
      archiveArtifacts artifacts: 'reports/allure-results/**', allowEmptyArchive: true
    }
  }
}
```

## Allure 3

Current Jenkins plugin syntax selects the generator explicitly:

```groovy
allure allureVersion: '3',
       includeProperties: false,
       results: [[path: 'reports/allure-results']]
```

The reporter’s default `allure2-and-3` compatibility mode writes the official Allure 2 result format, which Allure 3 consumes. Use one clean result directory per Jenkins build; do not merge mutable directories from concurrent jobs. `executor.json` is auto-populated from Jenkins variables such as `BUILD_URL`, `BUILD_TAG`, `JOB_NAME`, and `BUILD_NUMBER` when present.

See [examples/jenkins/Jenkinsfile](../examples/jenkins/Jenkinsfile). The pipeline fragment follows the current official Jenkins integration examples researched on 2026-08-07.
