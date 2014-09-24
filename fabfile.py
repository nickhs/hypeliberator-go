from fabric.api import run, local, env
from fabric.contrib.project import rsync_project

env.hosts = ["nickhs"]
env.use_ssh_config = True


def setup():
    run("mkdir -p /srv/hypeliberator-go")
    run("mkdir -p /var/log/hypeliberator-go")
    rsync_project("/etc/supervisor/conf.d/hypeliberator-go.conf", "ops/supervisor.conf")


def deploy():
    local('gox -osarch="linux/amd64" -output hypeliberator-go')
    rsync_project("/srv/hypeliberator-go/main", "hypeliberator-go")
    rsync_project("/srv/hypeliberator-go/index.html", "index.html")
    rsync_project("/srv/hypeliberator-go/static", "static/")
    local("rm hypeliberator-go")
    run("sudo supervisorctl restart hypeliberator-go")
