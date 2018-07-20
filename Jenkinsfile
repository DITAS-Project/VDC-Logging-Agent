pipeline {
    agent any   
    stages {
         stage('Image creation') {
            steps {
                echo 'Creating the image...'
                // This will search for a Dockerfile in the src folder and will build the image to the local repository
                // Using latest tag to override tha newest image in the hub
                sh "docker build -t \"ditas/vdc-logging-agent:latest\" ."
                echo "Done"
            }
        }
        stage('Push image') {
            steps {
                echo 'Retrieving Docker Hub password from /opt/ditas-docker-hub.passwd...'
                // Get the password from a file. This reads the file from the host, not the container. Slaves already have the password in there.
                script {
                    password = readFile '/opt/ditas-docker-hub.passwd'
                }
                echo "Done"
                // Login to DockerHub with the ditas generic Docker Hub user
                sh "docker login -u ditasgeneric -p ${password}"
                echo 'Login to Docker Hub as ditasgeneric...'
                sh "docker login -u ditasgeneric -p ${password}"
                echo "Done"
                echo "Pushing the image ditas/vdc-logging-agent:latest..."
                // Push the image to DockerHub
                sh "docker push ditas/vdc-logging-agent:latest"
                echo "Done"
            }
        }
    }
}