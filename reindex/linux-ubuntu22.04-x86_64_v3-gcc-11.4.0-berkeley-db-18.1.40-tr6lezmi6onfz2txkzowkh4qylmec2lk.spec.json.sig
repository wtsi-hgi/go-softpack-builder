-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA512

{
"spec":{
"_meta":{
"version":4
},
"nodes":[
{
"name":"berkeley-db",
"version":"18.1.40",
"arch":{
"platform":"linux",
"platform_os":"ubuntu22.04",
"target":"x86_64_v3"
},
"compiler":{
"name":"gcc",
"version":"11.4.0"
},
"namespace":"builtin",
"parameters":{
"build_system":"autotools",
"cxx":true,
"docs":false,
"patches":[
"26090f418891757af46ac3b89a9f43d6eb5989f7a3dce3d1cfc99fba547203b3",
"b231fcc4d5cff05e5c3a4814f6a5af0e9a966428dc2176540d2c05aff41de522"
],
"stl":true,
"cflags":[],
"cppflags":[],
"cxxflags":[],
"fflags":[],
"ldflags":[],
"ldlibs":[]
},
"patches":[
"b231fcc4d5cff05e5c3a4814f6a5af0e9a966428dc2176540d2c05aff41de522",
"26090f418891757af46ac3b89a9f43d6eb5989f7a3dce3d1cfc99fba547203b3"
],
"package_hash":"h57ydfn33zevvzctzzioiiwjwe362izbbwncb6a26dfeno4y7tda====",
"dependencies":[
{
"name":"gmake",
"hash":"c2psokv7rk36ryfmmxysssisb2lyb43z",
"parameters":{
"deptypes":[
"build"
],
"virtuals":[]
}
}
],
"hash":"tr6lezmi6onfz2txkzowkh4qylmec2lk"
},
{
"name":"gmake",
"version":"4.3",
"arch":{
"platform":"linux",
"platform_os":"ubuntu22.04",
"target":"x86_64_v3"
},
"compiler":{
"name":"gcc",
"version":"11.4.0"
},
"namespace":"sanger.hgi",
"parameters":{
"build_system":"generic",
"guile":false,
"patches":[
"599f134e69f696ad1ff32b001ea132a49749ebd92ab2b4f4d26afdda2c33cc43"
],
"cflags":[],
"cppflags":[],
"cxxflags":[],
"fflags":[],
"ldflags":[],
"ldlibs":[]
},
"patches":[
"599f134e69f696ad1ff32b001ea132a49749ebd92ab2b4f4d26afdda2c33cc43"
],
"package_hash":"tkiqlaz7j6567htlpildu4vtx6tnj2yrrwx4h2uyqqx7eccwtfma====",
"hash":"c2psokv7rk36ryfmmxysssisb2lyb43z"
}
]
},
"buildcache_layout_version":1,
"binary_cache_checksum":{
"hash_algorithm":"sha256",
"hash":"ba759ad5308f65fa28feb5e9f8ae11361fba5521a715ca772011975a30742022"
}
}
-----BEGIN PGP SIGNATURE-----

iQIzBAEBCgAdFiEEIBrF+U5viHQFgSEyP7lSffSsYJcFAmWyTOAACgkQP7lSffSs
YJfJ4RAAot7fHptmwpDagtszw0MfqFWra0o33Q6ViXN2BaQo3meJcEfgZwbVcJmT
G24GnUiJIJkXyaKXjV6chkM44M4QSbk197X9wMn65kLuKjSOWA21YPevLvD6maD/
t9js1JnWNmw7YnpUQDENQMXaxJl6Ke1Km2lRhjfV940VB+ldiOakzy1cQWmGGCGp
yi8cDit/Z+ppBkH29Njb153X0tsdPYHC0+0py8DDshZdvRsD/lBDM+AhfXo55ntm
qI+b4mjtsj7veFTISpCc9rIiwTSgK7TpqmFnR/KhsmSQBlRVOCVk8lXyDzJmmDpE
g8y/+EJUZDThsvccM9A0mNd4l5fZDkL4cwkda7fyqb46UEcS4WM7tKWx2wKWxJwv
6BwZIV504z+QvYXtqFGh+YeaqoQ3rRux7pubCeIsUwkKXnQN7aYCHw61CXuEN80d
1fOxOiLSJ9TvoHW6vbvH/ItJeQx3fC8VppyF+2Z7F1bs67uXVXvN6mSqhzYNIoMl
Ko33AFVeAmKS2T71iavkNhdSPvdar8c7JuU10sdv75244Ss7tUVOA3GvuDjovrAg
tCN8/0QX7SfnO4dGJ0CmMWUqJhGr86V3mq+RGt3pIXkvzZoyO6HiAbJyQnenRqlS
dKFt0RQ2rhtu0zqnBjE9/j8TBIdGhSf+Ejvi4vM4yZSz8Gi/30s=
=2bzr
-----END PGP SIGNATURE-----
