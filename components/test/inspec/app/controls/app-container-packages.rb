# @file app-container-packages.rb
# @brief Verify the pacakge installations inside the Docker container.
#
# @description This Inspec Module verifies the pacakge installations inside the Docker container.
#
# This module adopts tests from https://dev-sec.io/baselines/linux.

title "App Container Package Installations"

control 'package-01' do
    impact 1.0
    title 'Do not run deprecated inetd or xinetd'
    desc 'http://www.nsa.gov/ia/_files/os/redhat/rhel5-guide-i731.pdf, Chapter 3.2.1'

    describe package('inetd') do
        it { should_not be_installed }
    end

    describe package('xinetd') do
        it { should_not be_installed }
    end
end

control 'package-02' do
    impact 1.0
    title 'Do not install Telnet server'
    desc 'Telnet protocol uses unencrypted communication, that means the password and other sensitive data are unencrypted. http://www.nsa.gov/ia/_files/os/redhat/rhel5-guide-i731.pdf, Chapter 3.2.2'

    describe package('telnetd') do
        it { should_not be_installed }
    end
end

control 'package-03' do
    impact 1.0
    title 'Do not install rsh server'
    desc 'The r-commands suffers same problem as telnet. http://www.nsa.gov/ia/_files/os/redhat/rhel5-guide-i731.pdf, Chapter 3.2.3'

    describe package('rsh-server') do
        it { should_not be_installed }
    end
end

control 'package-05' do
    impact 1.0
    title 'Do not install ypserv server (NIS)'
    desc 'Network Information Service (NIS) has some security design weaknesses like inadequate protection of important authentication information. http://www.nsa.gov/ia/_files/os/redhat/rhel5-guide-i731.pdf, Chapter 3.2.4'

    describe package('ypserv') do
        it { should_not be_installed }
    end
end

control 'package-06' do
    impact 1.0
    title 'Do not install tftp server'
    desc 'tftp-server provides little security http://www.nsa.gov/ia/_files/os/redhat/rhel5-guide-i731.pdf, Chapter 3.2.5'

    describe package('tftp-server') do
        it { should_not be_installed }
    end
end

control 'package-09' do
    impact 1.0
    title 'CIS: Additional process hardening'
    desc '1.5.4 Ensure prelink is disabled'

    describe package('prelink') do
        it { should_not be_installed }
    end
end
