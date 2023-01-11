rmdir /s /q out

mkdir out
mkdir out\QueryService
mkdir out\QueryService\Code

copy sfpkg\ApplicationManifest.xml out\ApplicationManifest.xml
copy sfpkg\QueryService\ServiceManifest.xml out\QueryService\ServiceManifest.xml

cd code

go build -o ..\out\QueryService\Code\QueryService.exe

cd ..