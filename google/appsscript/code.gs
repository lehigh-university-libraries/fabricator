function onOpen() {
  var ui = SpreadsheetApp.getUi();
  ui.createMenu('Lehigh Preserve')
    .addItem('Check My Work', 'sendSheetData')
    .addToUi();
}

function sendSheetData() {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getActiveSheet();
  var data = sheet.getDataRange().getValues();

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
  var result = JSON.parse(response.getContentText());

  displayErrors(result);
}

function displayErrors(e) {
  if (e.length == 0) {
    SpreadsheetApp.getUi().alert('Looks good! 🚀');
    return;
  }

  var sheet = SpreadsheetApp.getActiveSpreadsheet().getActiveSheet();

  for (var cell in e) {
    var error = e[cell];
    sheet.getRange(cell).setBackground('red').setNote(error);
  }

  SpreadsheetApp.getUi().alert('Errors highlighted in the sheet.');
}
