# @file app-container-filesystem.rb.rb
# @brief Verify files and folders
#
# @description This Inspec Module verifies files and folders containing the webserver
# configuration and the website content. That includes that no artifacts from intermediate
# stages should be present in the final image (other than the website itself).

title "Verify files and folders"

control "app-folders-and-files" do
    impact 0.7
    title "Check existence of mandatory files and folders"
    desc "Check files and folders which either contain binaries or config files mandatory to run the app"

    FOLDERS = %w(
        /opt/app
    )
    FOLDERS.each do |folder|
        describe file(folder) do
            it { should exist }
            it { should_not be_file }
            it { should be_directory }
        end
    end

    FILES = %w(
        /opt/app/app.jar
    )
    FILES.each do |file|
        describe file(file) do
            it { should exist }
            it { should be_file }
            it { should_not be_directory }
        end
    end

end
