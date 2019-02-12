pipeline {
    environment {
      TAG = "master"
    }
    agent any   
    stages {
        stage('Image creation') {
            steps {
                echo 'Creating the image...'
                sh "docker build -f Dockerfile.testing -t \"ditas/vdc-logging-agent:testing\" . --no-cache"
                sh "docker build -f Dockerfile.artifact -t \"ditas/vdc-logging-agent:${TAG}\" . --no-cache"
                echo "Done"
            }
        }
        stage('Testing'){
            options {
                // Don't need to checkout Git again
                skipDefaultCheckout true
            }
            steps{
                sh "docker run --rm ditas/vdc-logging-agent:testing go test ./..."
            }
        }
        stage('Push image') {
            options {
                // Don't need to checkout Git again
                skipDefaultCheckout true
            }
            steps {
                echo 'Retrieving Docker Hub password from /opt/ditas-docker-hub.passwd...'
        
                script {
                    password = readFile '/opt/ditas-docker-hub.passwd'
                }
                echo "Done"
               
                sh "docker login -u ditasgeneric -p ${password}"
                echo 'Login to Docker Hub as ditasgeneric...'
                sh "docker login -u ditasgeneric -p ${password}"
                echo "Done"
                echo "Pushing the image ditas/vdc-logging-agent:${TAG}..."
                
                sh "docker push ditas/vdc-logging-agent:${TAG}"
                echo "Done"
            }
        }
        stage('Deployment in Staging') {
            options {
                // Don't need to checkout Git again
                skipDefaultCheckout true
            }
            steps {
                sh './jenkins/deploy-staging.sh'
            }
        }
        stage('Dredd API validation in Staging') {
            agent any
            steps {
                sh './jenkins/dredd.sh'
            }
        }		
        stage('Production image creation and push') {
            when {
                expression {
                   // only create production image from master branch
                   branch 'master'
                }
            }
            steps {                
                // Change the tag from staging to production 
                sh "docker tag ditas/vdc-logging-agent:${TAG} ditas/vdc-logging-agent:production"
                sh "docker push ditas/vdc-logging-agent:production"
            }
        }

    }
}
