# @file app-container.rb
# @brief Verify the Docker image which contains the main application of the Fantasy Hockey Game.
#
# @description This Inspec Module verifies the operating system and configuration of the Docker
# container. The basic Docker container setup is checked as well.
#
# This module adopts tests from https://dev-sec.io/baselines/linux.

title "App Container Setup"

control "docker-image-config" do
    impact 0.5
    title "Verify operating system"
    desc "Verify if the operating system type and configuration"

    describe os.family do
        it { should eq 'debian' }
    end
end

control "docker-user-config" do
    impact 0.8
    title "Container Configuration Validation"
    desc "Verify the configuration settings within the Docker container"

    describe user('root') do
        it { should exist }
        it { should have_home_directory '/root' }
        it { should have_login_shell '/bin/bash' }
    end

    describe user('fantasy-hockey') do
        it { should exist }
        it { should have_home_directory '/home/fantasy-hockey' }
        it { should have_login_shell '/bin/bash' }
    end
end

shadow_group = 'root'
shadow_group = 'shadow' if os.debian? || os.suse? || os.name == 'alpine'

control 'os-01' do
    impact 1.0
    title 'Trusted hosts login'
    desc "hosts.equiv file is a weak implementation of authentication. Disabling the hosts.equiv support helps to prevent users from subverting the system's normal access control mechanisms of the system."

    describe file('/etc/hosts.equiv') do
        it { should_not exist }
    end
end

control 'os-02' do
    impact 1.0
    title 'Check owner and permissions for /etc/shadow'
    desc 'Check periodically the owner and permissions for /etc/shadow'

    describe file('/etc/shadow') do
        it { should exist }
        it { should be_file }
        it { should be_owned_by 'root' }
        its('group') { should eq shadow_group }
        it { should_not be_executable }
        it { should_not be_readable.by('other') }
    end

    if os.redhat? || os.name == 'fedora'
        describe file('/etc/shadow') do
            it { should_not be_writable.by('owner') }
            it { should_not be_readable.by('owner') }
        end
    else
        describe file('/etc/shadow') do
            it { should be_writable.by('owner') }
            it { should be_readable.by('owner') }
        end
    end

    if os.debian? || os.suse?
        describe file('/etc/shadow') do
            it { should be_readable.by('group') }
        end
    else
        describe file('/etc/shadow') do
            it { should_not be_readable.by('group') }
        end
    end
end

control 'os-03' do
    impact 1.0
    title 'Check owner and permissions for /etc/passwd'
    desc 'Check periodically the owner and permissions for /etc/passwd'

    describe file('/etc/passwd') do
        it { should exist }
        it { should be_file }
        it { should be_owned_by 'root' }
        its('group') { should eq 'root' }
        it { should_not be_executable }
        it { should be_writable.by('owner') }
        it { should_not be_writable.by('group') }
        it { should_not be_writable.by('other') }
        it { should be_readable.by('owner') }
        it { should be_readable.by('group') }
        it { should be_readable.by('other') }
    end
end

control 'os-03b' do
    impact 1.0
    title 'Check passwords hashes in /etc/passwd'
    desc 'Check periodically that /etc/passwd does not contain passwords'

    describe passwd do
        its('passwords') { should be_in ['x', '*'] }
    end
end

control 'os-04' do
    impact 1.0
    title 'Dot in PATH variable'
    desc 'Do not include the current working directory in PATH variable. This makes it easier for an attacker to gain extensive rights by executing a Trojan program'

    describe os_env('PATH') do
        its('split') { should_not include('') }
        its('split') { should_not include('.') }
    end
end

control 'os-07' do
    impact 1.0
    title 'Unique uid and gid'
    desc 'Check for unique uids gids'

    describe passwd do
        its('uids') { should_not contain_duplicates }
    end

    describe etc_group do
        its('gids') { should_not contain_duplicates }
    end
end

control 'os-09' do
    impact 1.0
    title 'Check for .rhosts and .netrc file'
    desc 'Find .rhosts and .netrc files - CIS Benchmark 9.2.9-10'
    output = command('find / -maxdepth 3 \( -iname .rhosts -o -iname .netrc \) -print 2>/dev/null | grep -v \'^find:\'')
    out = output.stdout.split(/\r?\n/)

    describe out do
        it { should be_empty }
    end
end

control 'os-11' do
    impact 1.0
    title 'Protect log-directory'
    desc 'The log-directory /var/log should belong to root'

    describe file('/var/log') do
        it { should be_directory }
        it { should be_owned_by 'root' }
        its(:group) { should match(/^root|syslog$/) }
    end
end

control 'os-13' do
    impact 1.0
    title 'Protect cron directories and files'
    desc 'The cron directories and files should belong to root.'

    cron_files = ['/etc/crontab', '/etc/cron.hourly', '/etc/cron.daily', '/etc/cron.weekly', '/etc/cron.monthly', '/etc/cron.d']

    cron_files.each do |cron_file|
        next unless file(cron_file).exist?

        describe file(cron_file) do
            it { should be_owned_by 'root' }
            it { should_not be_writable.by('group') }
            it { should_not be_writable.by('other') }
            it { should be_readable.by('group') }
            it { should be_readable.by('other') }
        end
    end
end
