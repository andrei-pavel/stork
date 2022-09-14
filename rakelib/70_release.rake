# Release
# This file contains the release-stage tasks.

# Establish Stork version
stork_version = '0.0.0'
version_file = 'backend/version.go'
text = File.open(version_file).read
text.each_line do |line|
    if line.start_with? 'const Version'
        parts = line.split('"')
        stork_version = parts[1]
    end
end
STORK_VERSION = stork_version


namespace :release do
    desc 'Generic task for bumping up version
        VERSION - target version after bump - required
    '
    task :bump do
        if ENV["VERSION"].nil?
            fail "Environment variable VERSION is not specified"
        end

        # Replace version in all files. Use specific patterns for each for stricter matching.
        for i in [
            ['api/swagger.in.yaml', "version: #{STORK_VERSION}", "version: #{ENV["VERSION"]}"],
            ['backend/version.go', "const Version = \"#{STORK_VERSION}\"", "const Version = \"#{ENV["VERSION"]}\""],
            ['webui/package.json', "\"version\": \"#{STORK_VERSION}\"", "\"version\": \"#{ENV["VERSION"]}\""],
            ['webui/package-lock.json', "\"version\": \"#{STORK_VERSION}\"", "\"version\": \"#{ENV["VERSION"]}\""],
        ] do
            File.open(i[0], 'r') do |file_r|
                contents = file_r.read
                contents.sub!(i[1], i[2])
                File.open(i[0], 'w') do |file_w|
                    file_w.write(contents)
                end
            end
        end

        # Announce release in ChangeLog.
        days_to_add = (3 + 7 - Date.today.wday) % 7
        next_wednesday = Date.today + days_to_add
        File.open('ChangeLog.md', 'r') do |file_r|
            contents = file_r.read
            contents = "Stork #{ENV["VERSION"]} released on #{next_wednesday}.\n\n" + contents
            File.open('ChangeLog.md', 'w') do |file_w|
                file_w.write(contents)
            end
        end

        # Put out an informative message that the bump was successful.
        puts "Version succesfully bumped to #{ENV["VERSION"]}."
    end


    namespace :bump do
        desc 'Bump up major version'
        task :major do
            major = STORK_VERSION.split('.')[0]
            major = Integer(major) + 1
            ENV["VERSION"] = "#{major}.0.0"
            Rake::Task['release:bump'].invoke()
        end

        desc 'Bump up minor version'
        task :minor do
            major = STORK_VERSION.split('.')[0]
            minor = STORK_VERSION.split('.')[1]
            minor = Integer(minor) + 1
            ENV["VERSION"] = "#{major}.#{minor}.0"
            Rake::Task['release:bump'].invoke()
        end

        desc 'Bump up patch version'
        task :patch do
            major = STORK_VERSION.split('.')[0]
            minor = STORK_VERSION.split('.')[1]
            patch = STORK_VERSION.split('.')[2]
            patch = Integer(patch) + 1
            ENV["VERSION"] = "#{major}.#{minor}.#{patch}"
            Rake::Task['release:bump'].invoke()
        end
    end


    desc 'Prepare release notes'
    task :notes do
        sh 'rm -rf stork.wiki'
        sh 'git clone --depth 1 https://gitlab.isc.org/isc-projects/stork.wiki.git'
        sh "cat './stork.wiki/Releases/Release-notes-#{STORK_VERSION}.md' |
            sed '/^```/d' | sed 's/\\\[/[/g;s/\\\]/]/g' |
            perl -pe 's|\[(http.*?)\]\(http.*\)|\1|' |
            fold -sw 73 > Stork-#{STORK_VERSION}-ReleaseNotes.txt"
        sh 'rm -rf stork.wiki'
    end

    desc 'Prepare release tarball with Stork sources'
    task :tarball do
        sh "git", "archive",
            "--prefix", "stork-#{STORK_VERSION}/",
            "-o", "stork-#{STORK_VERSION}.tar.gz", "HEAD"
    end
    CLEAN.append *FileList["stork-*.tar.gz"]


    namespace :tarball do
        desc 'Upload tarball and release notes to given host and path'
        task :upload, [:host, :path] do |t, args|
            path = "#{args[:path]}/#{STORK_VERSION}"
            sh "ssh -4 #{args[:host]} -- mkdir -p '#{path}'"
            sh "scp -4 -p \
                       './stork-#{STORK_VERSION}.tar.gz' \
                       './Stork-#{STORK_VERSION}-ReleaseNotes.txt' \
                       '#{args[:host]}:#{path}'"
            sh "ssh -4 #{args[:host]} -- chmod -R g+w #{path}"
        end
    end
end

namespace :check do
    desc 'Check the external dependencies related to the distribution'
    task :release do
        check_deps(__FILE__, "git")
    end
end

