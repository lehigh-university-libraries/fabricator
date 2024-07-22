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

  var options = {
    'method': 'POST',
    'contentType': 'application/json',
    'payload': payload
  };

  var response = UrlFetchApp.fetch(url, options);
  var result = JSON.parse(response.getContentText());

  displayErrors(result);
}

function displayErrors(errors) {
  var sheet = SpreadsheetApp.getActiveSpreadsheet().getActiveSheet();

  for (var cell in errors) {
    var error = errors[cell];
    sheet.getRange(cell).setBackground('red').setNote(error);
  }

  SpreadsheetApp.getUi().alert('Errors highlighted in the sheet.');
}
