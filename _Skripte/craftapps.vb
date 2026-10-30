#use json,net,string,zip
<<<<<<< HEAD

Dim repos = {"storytold/photocraft","storytold/filmcraft"}
Dim config="/volume2/homes/cb/craft.txt"
=======
'Benötigt: zip plugin. Im gleichen Unterverzeichnis plugin von vbxc 

Dim repos = {"storytold/photocraft","storytold/filmcraft"}
Dim config="<path>/craft.txt"
>>>>>>> 1a75fee5075b59ae56e731fda08bf18a063c2ad9
Dim photo=file.ReadValue(config,"photo=")
Dim film=file.ReadValue(config,"film=")
Dim webpath=file.ReadValue(config,"webpath=")
Dim zippath=file.ReadValue(config,"zippath=")

For Each repo in repos

    Print("Prüfe Release: " & repo)

    Dim response = net.Get("https://api.github.com/repos/" & repo & "/releases/latest")
    Dim latestVersion = Replace(json.Search(response,"tag_name"),"v","")

    If IsError(response) Then
        Print(ErrorText(response))
        Exit
    End If

    Dim downloadUrl = "",zipFile = "",targetDir = ""

    If repo = "storytold/photocraft" Then
        If string.StrComp(latestVersion,photo)=1 or photo="" then
            zipFile = zippath & "photocraft.zip"
            targetDir = webpath & "photocraft"
            downloadUrl = json.SearchDeepWhereGetContains(response,"browser_download_url","name~photocraft-web-" & Replace(json.Search(response,"tag_name"),"v","") & ".zip")
            Download(downloadUrl,targetDir,zipFile)
            file.UpdateValue(config,"photo=","photo=" & latestVersion)
        Else
            Print "Aktuelle Version schon vorhanden"
        End If
    ElseIf repo = "storytold/filmcraft" Then
        If string.StrComp(latestVersion,film)=1 or film="" then
            zipFile = zippath & "filmcraft.zip"
            targetDir = webpath & "filmcraft"
            downloadUrl = json.SearchDeepWhereGetContains(response,"browser_download_url","name~filmcraft-web-" & Replace(json.Search(response,"tag_name"),"v","") & ".zip")
            Download(downloadUrl,targetDir,zipFile)
            file.UpdateValue(config,"film=","film=" & latestVersion)
        Else
            Print "Aktuelle Version schon vorhanden"
        End If
        ' ElseIf repo = "storytold/pdfcraft" Then
        '    zipFile = "c:\test\pdfcraft.zip"
        '   targetDir = "c:\test\pdfcraft"
    End If
Next

Sub Download(downloadUrl,targetDir,zipFile)
    Print("Download: " & downloadUrl)
    Dim result = ""

    result = net.Download(downloadUrl, zipFile)

    If IsError(result) Then
        Print("Download fehlgeschlagen: " & ErrorText(result))
    End If

    Print("Entpacke nach: " & targetDir)

    folder.Delete(targetDir)

    result = zip.Extract(zipFile, targetDir,1)

    If IsError(result) Then
        Print("Entpacken fehlgeschlagen: " & ErrorText(result))
    End If
    file.Delete(zipFile)
    Print("Erfolgreich abgeschlossen: " & repo)
End Sub