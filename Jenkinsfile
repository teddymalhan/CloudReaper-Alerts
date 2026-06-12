pipeline {
    agent any

    environment {
        AWS_ENDPOINT_URL      = 'http://localhost:4566'
        AWS_ACCESS_KEY_ID     = 'test'
        AWS_SECRET_ACCESS_KEY = 'test'
        AWS_DEFAULT_REGION    = 'us-east-1'
        // For a real Slack message, expose SLACK_BOT_TOKEN / SLACK_CHANNEL_ID via Jenkins
        // credentials. Otherwise the pipeline provisions with harmless placeholders.
        SLACK_BOT_TOKEN  = "${env.SLACK_BOT_TOKEN ?: 'xoxb-localstack-placeholder'}"
        SLACK_CHANNEL_ID = "${env.SLACK_CHANNEL_ID ?: 'C0000000000'}"
    }

    stages {
        stage('Start Floci') {
            steps {
                sh '''
                    if ! curl -s -o /dev/null "$AWS_ENDPOINT_URL" 2>/dev/null; then
                      docker compose up -d floci
                    fi
                    for _ in $(seq 1 45); do
                      curl -s -o /dev/null "$AWS_ENDPOINT_URL" 2>/dev/null && break
                      sleep 2
                    done
                '''
            }
        }

        stage('Provision') {
            steps {
                sh '''
                    make notifier

                    terraform -chdir=terraform init -input=false
                    terraform -chdir=terraform apply -auto-approve -input=false \
                      -var "slack_bot_token=$SLACK_BOT_TOKEN" \
                      -var "slack_channel_id=$SLACK_CHANNEL_ID"
                '''
            }
        }

        stage('Detect Orphans') {
            steps {
                script {
                    sh 'make detector'
                    def rc = sh(
                        returnStatus: true,
                        script: './build/detector -report build/report.json -markdown build/report.md'
                    )
                    // detector exits 1 when orphans are found
                    env.ORPHANS_FOUND = (rc != 0) ? 'true' : 'false'
                }
            }
        }
    }

    post {
        always {
            archiveArtifacts artifacts: 'build/report.*', allowEmptyArchive: true
            script {
                if (env.ORPHANS_FOUND == 'true') {
                    echo 'Orphans found — sending Slack alert through the pipeline'
                    sh '''
                        QUEUE_URL=$(terraform -chdir=terraform output -raw main_queue_send_url)
                        make sender
                        AWS_ENDPOINT_URL="$AWS_ENDPOINT_URL" ./build/sender \
                          -report build/report.json -queue-url "$QUEUE_URL" -build-url "$BUILD_URL"
                    '''
                } else {
                    echo 'No orphans found — no Slack notification sent.'
                }
            }
            // Floci is a persistent compose service — left running between builds.
        }
    }
}
