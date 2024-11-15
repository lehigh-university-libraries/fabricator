function onOpen() {
  var ui = SpreadsheetApp.getUi();
  ui.createMenu('Lehigh Preserve')
    .addItem('Open Contributor Form', 'showForm')
    .addItem('Check My Work', 'sendSheetData')
    .addItem('Start workbench ingest', 'openExternalLink')
    .addToUi();
}

function openExternalLink() {
  var html = HtmlService.createHtmlOutput(
    '<p><a href="https://github.com/login?return_to=https%3A%2F%2Fgithub.com%2Flehigh-university-libraries%2Ffabricator%2Factions%2Fworkflows%2Frun.yml" target="_blank">Go to GitHub</a> to start a workbench ingest.</p><p>Make sure to copy the URL of this sheet to start the ingest.</p>'
  ).setWidth(300).setHeight(120);
  SpreadsheetApp.getUi().showModalDialog(html, 'Start workbench ingest');
}

function sendSheetData() {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getActiveSheet();
  var data = sheet.getDataRange().getValues();
  for (var i = 0; i < data.length; i++) {
    for (var j = 0; j < data[i].length; j++) {
      data[i][j] = data[i][j].toString();
    }
  }
  var payload = JSON.stringify(data);
  var url = 'https://preserve.lehigh.edu/workbench/check';
  const oauthToken = ScriptApp.getIdentityToken();
  var options = {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer ' + oauthToken
    },
    contentType: 'application/json',
    payload: payload
  };

  var lastRow = sheet.getLastRow();
  var lastColumn = sheet.getLastColumn();
  var range = sheet.getRange(2, 1, lastRow - 1, lastColumn); // A2 to last cell
  range.setBackground(null);
  range.clearNote();

  var response = UrlFetchApp.fetch(url, options);
  var t = response.getContentText()
  if (t.length == 2) {
    SpreadsheetApp.getUi().alert('Looks good! 🚀');
    return;
  }
  var result = JSON.parse(t);
  displayErrors(result);
}

function displayErrors(e) {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getActiveSheet();

  var count = 0;
  for (var cell in e) {
    var error = e[cell];
    sheet.getRange(cell).setBackground('red').setNote(error);
    count += 1;
  }

  SpreadsheetApp.getUi().alert('Found ' + count + ' errors highlighted in the sheet.');
}
