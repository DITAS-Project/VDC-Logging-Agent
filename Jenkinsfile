pipeline {
    environment {
      TAG = "v02"
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
            steps{
                sh "docker run --rm ditas/vdc-logging-agent:testing go test ./..."
            }
        }
        stage('Push image') {
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
    }
}