# System tests
# Run the system tests using docker-compose

#############
### Files ###
#############

# The files generated once by this script
autogenerated = []

# The autogenerated files mounted as volumes
volume_files = []

system_tests_dir = "tests/system"
kea_many_subnets_dir = "tests/system/config/kea-many-subnets"
directory kea_many_subnets_dir
kea_many_subnets_config_file = File.join(kea_many_subnets_dir, "kea-dhcp4.conf")
file kea_many_subnets_config_file => [kea_many_subnets_dir] do
    sh PYTHON, "docker/tools/gen-kea-config.py", "7000",
        "-o", kea_many_subnets_config_file,
        "--interface", "eth1"
end
autogenerated.append kea_many_subnets_dir, kea_many_subnets_config_file
volume_files.append kea_many_subnets_config_file

# These files are generated by the system tests but must exist initially.
lease4_file = "tests/system/config/kea/kea-leases4.csv"
lease6_file = "tests/system/config/kea/kea-leases6.csv"

file lease4_file do
    sh "touch", lease4_file
end

file lease6_file do
    sh "touch", lease6_file
end

CLEAN.append lease4_file, lease6_file
volume_files.append lease4_file, lease6_file

# TLS credentials
tls_dir = "tests/system/config/certs"
cert_file = File.join(tls_dir, "cert.pem")
key_file = File.join(tls_dir, "key.pem")
ca_dir = File.join(tls_dir, "CA")
directory ca_dir

file cert_file => [ca_dir] do
    sh "openssl", "req", "-x509", "-newkey", "rsa:4096",
        "-sha256", "-days", "3650", "-nodes",
        "-keyout", key_file, "-out", cert_file,
        "-subj", "/CN=kea.isc.org", "-addext",
        "subjectAltName=DNS:kea.isc.org,DNS:www.kea.isc.org,IP:127.0.0.1"
end
file key_file => [cert_file]
autogenerated.append cert_file, key_file, ca_dir
volume_files.append cert_file, key_file, ca_dir

# Server API
open_api_generator_python_dir = "tests/system/openapi_client"
file open_api_generator_python_dir => [SWAGGER_FILE, OPENAPI_GENERATOR] do
    sh "rm", "-rf", open_api_generator_python_dir
    sh "java", "-jar", OPENAPI_GENERATOR, "generate",
        "-i", SWAGGER_FILE,
        "-g", "python",
        "-o", "tests/system",
        "--global-property", "apiTests=false,modelTests=false",
        "--additional-properties", "generateSourceCodeOnly=true"
    sh "touch", open_api_generator_python_dir
end
autogenerated.append open_api_generator_python_dir
CLEAN.append "tests/system/.openapi-generator",
    "tests/system/.openapi-generator", "tests/system/openapi_client_README.md",
    "tests/system/.openapi-generator-ignore",  *FileList["tests/system/**/__pycache__"],
    *FileList["tests/system/**/.pytest_cache"]

CLEAN.append *autogenerated

# The system tests log directories
CLEAN.append "test-results/", "tests/system/test-results/"

#########################
### System test tasks ###
#########################

desc 'Run system tests
    TEST - Name of the test to run - optional'
task :systemtest => [PYTEST, open_api_generator_python_dir, *volume_files] do
    opts = []

    if !ENV["TEST"].nil?
        opts.append "-k", ENV["TEST"]
    end

    Dir.chdir(system_tests_dir) do
        sh PYTEST, "-s", *opts
    end
end

namespace :systemtest do

    desc 'List the test cases'
    task :list => [PYTEST, open_api_generator_python_dir] do
        Dir.chdir(system_tests_dir) do
            sh PYTEST, "--collect-only"
        end
    end

    desc 'Build the containers used in the system tests'
    task :build do
        Rake::Task["systemtest:sh"].invoke("build")
    end

    desc 'Run shell in the docker-compose container
        SERVICE - name of the docker-compose service - required
        SERVICE_USER - user to log in - optional'
    task :shell do
        user = []
        if !ENV["SERVICE_USER"].nil?
            user.append "--user", ENV["SERVICE_USER"]
        end

        Rake::Task["systemtest:sh"].invoke(
            "exec", *user, ENV["SERVICE"], "/bin/sh")
    end

    desc 'Display docker-compose logs
        SERVICE - name of the docker-compose service - optional'
    task :logs do
        service_name = []
        if !ENV["SERVICE"].nil?
            service_name.append ENV["SERVICE"]
        end
        Rake::Task["systemtest:sh"].invoke("logs", *service_name)
    end

    desc 'Run perfdhcp docker-compose service'
    task :perfdhcp do |t, args|
        Rake::Task["systemtest:sh"].invoke("run", "perfdhcp", *args)
    end

    desc 'Run system tests docker-compose
        USE_BUILD_KIT - use BuildKit for faster build - default: true
        CS_REPO_ACCESS_TOKEN - build the containers including Kea premium features - optional
    '
    task :sh => volume_files do |t, args|
        if ENV["USE_BUILD_KIT"] != "false"
            ENV["COMPOSE_DOCKER_CLI_BUILD"] = "1"
            ENV["DOCKER_BUILDKIT"] = "1"
        end

        ENV["PWD"] = Dir.pwd

        profiles = []
        if !ENV["CS_REPO_ACCESS_TOKEN"].nil?
            puts "Use the Kea premium containers"
            profiles.append "--profile", "premium"
        end

        sh "docker-compose",
            "-f", File.expand_path(File.join(system_tests_dir, "docker-compose.yaml")),
            "--project-directory", File.expand_path("."),
            "--project-name", "stork_tests",
            *profiles,
            *args
    end

    desc 'Create autogenerated configs'
    task :gen do
        autogenerated.each do |f|
            Rake::FileTask[f].invoke()
        end
    end

    desc 'Recreate autogenerated configs'
    task :regen do
        autogenerated.each do |f|
            FileUtils.rm_rf(f)
        end

        Rake::Task["systemtest:gen"].invoke()
    end

    desc 'Down all running services, removes networks and volumes'
    task :down do
        Rake::Task["systemtest:sh"].invoke("down", "--volumes", "--remove-orphans")
    end

end

namespace :prepare do
    desc 'Install the external dependencies related to the system tests'
    task :systemtest do
        find_and_prepare_deps(__FILE__)
    end
end

namespace :check do
    desc 'Check the external dependencies related to the system tests'
    task :systemtest do
        check_deps(__FILE__, "python3", "docker", "docker-compose",
            "openssl", "java")
    end
end
