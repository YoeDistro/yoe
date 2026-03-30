def user(name, uid, gid, home = "", shell = "/bin/sh", password = "", gecos = ""):
    """Returns a dict describing a user account.

    Args:
        name: login name
        uid: numeric user ID
        gid: numeric group ID
        home: home directory (default: /root for uid 0, /home/<name> otherwise)
        shell: login shell (default: /bin/sh)
        password: plaintext password (hashed at build time via openssl); empty = no password
        gecos: comment/full name field
    """
    if not home:
        if uid == 0:
            home = "/root"
        else:
            home = "/home/" + name
    return {
        "name": name,
        "uid": uid,
        "gid": gid,
        "home": home,
        "shell": shell,
        "password": password,
        "gecos": gecos,
    }

def users_commands(users):
    """Returns shell commands to populate /etc/passwd, /etc/group, /etc/shadow."""
    cmds = [
        "true > $DESTDIR/etc/passwd",
        "true > $DESTDIR/etc/group",
        "true > $DESTDIR/etc/shadow",
        "chmod 0600 $DESTDIR/etc/shadow",
    ]
    for u in users:
        cmds.append(
            "echo '" + u["name"] + ":x:" + str(u["uid"]) + ":" +
            str(u["gid"]) + ":" + u["gecos"] + ":" + u["home"] + ":" +
            u["shell"] + "' >> $DESTDIR/etc/passwd",
        )
        cmds.append(
            "echo '" + u["name"] + ":x:" + str(u["gid"]) +
            ":' >> $DESTDIR/etc/group",
        )
        if u["password"]:
            cmds.append(
                "PW=$(LD_LIBRARY_PATH=/build/sysroot/usr/lib openssl passwd -6 '" +
                u["password"] + "') && " +
                "echo '" + u["name"] + ":'\"$PW\"':0:0:99999:7:::' >> $DESTDIR/etc/shadow",
            )
        else:
            cmds.append(
                "echo '" + u["name"] +
                "::0:0:99999:7:::' >> $DESTDIR/etc/shadow",
            )
        if u["uid"] != 0:
            cmds.append("mkdir -p $DESTDIR" + u["home"])
    for u in users:
        if u["uid"] == 0:
            cmds.append("chmod 0700 $DESTDIR" + u["home"])
    return cmds
