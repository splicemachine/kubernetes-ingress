# Splicemachine Fork of https://github.com/haproxytech/kubernetes-ingress

We will keep our local `master` branch in sync with the parent master, pulling down change into our repository.
We will create our master branch as `sm_master`.  We will merge changes from our local `master` down into `sm_master`
We will create working branches off of `sm_master` and merge our modifications into the `sm_master` branch.
Our customized Docker images will be built from the `sm_master` branch.

## Jenkins Docker Build

The Jenkins file has references to the current upstream version of haproxy (2.0.11) that we are building against.  If we pull down
a newer version that contains a newer version of haproxy, we should change these references and make.  This takes place
of `master-` naming we do for the docker images we own.

```bash
master_found = sh returnStdout: true, script: "wget -q https://hub.docker.com/v2/repositories/splicemachine/kubernetes-ingress/tags?page=$page_num -O - | jq '.' | grep 'name' | grep '2.0.11'"
```

We are also writing back to the dbaas-infrastructure `./kubernetes/charts/splice/values.yaml` file.

```bash
sh "sed -i '/repository: splicemachine\\/kubernetes-ingress/{N;s/tag: 2.0.11_.*\$/tag: 2.0.11_${existing_version_major}.${existing_version_minor}.${new_hotfix_version}/}' charts/splice/values.yaml"
```

The above SED requires these two lines be together so the replacement will work.  These appear under the `haproxy-controller:` section.

```yaml
      repository: splicemachine/kubernetes-ingress
      tag: 2.0.11_0.0.1
```
